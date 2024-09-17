package directvolumemigration

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/konveyor/crane-lib/state_transfer/transfer"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	prometheusapi "github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	virtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prometheusURLKey = "PROMETHEUS_URL"
	prometheusRoute  = "prometheus-k8s"
	progressQuery    = "kubevirt_vmi_migration_data_processed_bytes{name=\"%s\"} / (kubevirt_vmi_migration_data_processed_bytes{name=\"%s\"} + kubevirt_vmi_migration_data_remaining_bytes{name=\"%s\"}) * 100"
)

var (
	ErrVolumesDoNotMatch    = errors.New("volumes do not match")
	ErrNamespacesDoNotMatch = errors.New("source and target namespaces must match")
)

type vmVolumes struct {
	sourceVolumes []string
	targetVolumes []string
}

func (t *Task) startLiveMigrations(nsMap map[string][]transfer.PVCPair) ([]string, error) {
	reasons := []string{}
	vmVolumeMap := make(map[string]vmVolumes)
	sourceClient := t.sourceClient
	if t.Owner.IsRollback() {
		sourceClient = t.destinationClient
	}
	for k, v := range nsMap {
		namespace, err := getNamespace(k)
		if err != nil {
			reasons = append(reasons, err.Error())
			return reasons, err
		}
		volumeVmMap, err := getRunningVmVolumeMap(sourceClient, namespace)
		if err != nil {
			reasons = append(reasons, err.Error())
			return reasons, err
		}
		for _, pvcPair := range v {
			if vmName, found := volumeVmMap[pvcPair.Source().Claim().Name]; found {
				vmVolumeMap[vmName] = vmVolumes{
					sourceVolumes: append(vmVolumeMap[vmName].sourceVolumes, pvcPair.Source().Claim().Name),
					targetVolumes: append(vmVolumeMap[vmName].targetVolumes, pvcPair.Destination().Claim().Name),
				}
			}
		}
		for vmName, volumes := range vmVolumeMap {
			if err := t.storageLiveMigrateVM(vmName, namespace, &volumes); err != nil {
				switch err {
				case ErrVolumesDoNotMatch:
					reasons = append(reasons, fmt.Sprintf("source and target volumes do not match for VM %s", vmName))
					continue
				default:
					reasons = append(reasons, err.Error())
					return reasons, err
				}
			}
		}
	}
	return reasons, nil
}

func getNamespace(colonDelimitedString string) (string, error) {
	namespacePair := strings.Split(colonDelimitedString, ":")
	if len(namespacePair) != 2 {
		return "", fmt.Errorf("invalid namespace pair: %s", colonDelimitedString)
	}
	if namespacePair[0] != namespacePair[1] && namespacePair[0] != "" {
		return "", ErrNamespacesDoNotMatch
	}
	return namespacePair[0], nil
}

// Return a list of namespace/volume combinations that are currently in use by running VMs
func (t *Task) getRunningVMVolumes(namespaces []string) ([]string, error) {
	runningVMVolumes := []string{}

	for _, ns := range namespaces {
		volumesVmMap, err := getRunningVmVolumeMap(t.sourceClient, ns)
		if err != nil {
			return nil, err
		}
		for volume := range volumesVmMap {
			runningVMVolumes = append(runningVMVolumes, fmt.Sprintf("%s/%s", ns, volume))
		}
	}
	return runningVMVolumes, nil
}

func getRunningVmVolumeMap(client k8sclient.Client, namespace string) (map[string]string, error) {
	volumesVmMap := make(map[string]string)
	vmMap, err := getVMNamesInNamespace(client, namespace)
	if err != nil {
		return nil, err
	}
	for vmName := range vmMap {
		podList := corev1.PodList{}
		client.List(context.TODO(), &podList, k8sclient.InNamespace(namespace))
		for _, pod := range podList.Items {
			for _, owner := range pod.OwnerReferences {
				if owner.Name == vmName && owner.Kind == "VirtualMachineInstance" {
					for _, volume := range pod.Spec.Volumes {
						if volume.PersistentVolumeClaim != nil {
							volumesVmMap[volume.PersistentVolumeClaim.ClaimName] = vmName
						}
					}
				}
			}
		}
	}
	return volumesVmMap, nil
}

func getVMNamesInNamespace(client k8sclient.Client, namespace string) (map[string]interface{}, error) {
	vms := make(map[string]interface{})
	vmList := virtv1.VirtualMachineList{}
	err := client.List(context.TODO(), &vmList, k8sclient.InNamespace(namespace))
	if err != nil {
		if meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, vm := range vmList.Items {
		vms[vm.Name] = nil
	}
	return vms, nil
}

func (t *Task) storageLiveMigrateVM(vmName, namespace string, volumes *vmVolumes) error {
	sourceClient := t.sourceClient
	if t.Owner.IsRollback() {
		sourceClient = t.destinationClient
	}
	vm := &virtv1.VirtualMachine{}
	if err := sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: namespace, Name: vmName}, vm); err != nil {
		return err
	}
	// Check if the source volumes match before attempting to migrate.
	if !t.compareVolumes(vm, volumes.sourceVolumes) {
		// Check if the target volumes match, if so, the migration is already in progress or complete.
		if t.compareVolumes(vm, volumes.targetVolumes) {
			t.Log.V(5).Info("Volumes already updated for VM", "vm", vmName)
			return nil
		} else {
			return ErrVolumesDoNotMatch
		}
	}

	// Volumes match, create patch to update the VM with the target volumes
	return updateVM(sourceClient, vm, volumes.sourceVolumes, volumes.targetVolumes, t.Log)
}

func (t *Task) compareVolumes(vm *virtv1.VirtualMachine, volumes []string) bool {
	foundCount := 0
	if vm.Spec.Template == nil {
		return true
	}
	for _, vmVolume := range vm.Spec.Template.Spec.Volumes {
		if vmVolume.PersistentVolumeClaim == nil && vmVolume.DataVolume == nil {
			// Skip all non PVC or DataVolume volumes
			continue
		}
		if vmVolume.PersistentVolumeClaim != nil {
			if slices.Contains(volumes, vmVolume.PersistentVolumeClaim.ClaimName) {
				foundCount++
			}
		} else if vmVolume.DataVolume != nil {
			if slices.Contains(volumes, vmVolume.DataVolume.Name) {
				foundCount++
			}
		}
	}
	return foundCount == len(volumes)
}

func findVirtualMachineInstanceMigration(client k8sclient.Client, vmName, namespace string) (*virtv1.VirtualMachineInstanceMigration, error) {
	vmimList := &virtv1.VirtualMachineInstanceMigrationList{}
	if err := client.List(context.TODO(), vmimList, k8sclient.InNamespace(namespace)); err != nil {
		return nil, err
	}
	for _, vmim := range vmimList.Items {
		if vmim.Spec.VMIName == vmName {
			return &vmim, nil
		}
	}
	return nil, nil
}

func virtualMachineMigrationStatus(client k8sclient.Client, vmName, namespace string, log logr.Logger) (string, error) {
	log.Info("Checking migration status for VM", "vm", vmName)
	vmim, err := findVirtualMachineInstanceMigration(client, vmName, namespace)
	if err != nil {
		return err.Error(), err
	}
	if vmim != nil {
		log.V(5).Info("Found VMIM", "vmim", vmim.Name, "namespace", vmim.Namespace)
		if vmim.Status.MigrationState != nil {
			if vmim.Status.MigrationState.Failed {
				return vmim.Status.MigrationState.FailureReason, nil
			} else if vmim.Status.MigrationState.AbortStatus == virtv1.MigrationAbortSucceeded {
				return "Migration canceled", nil
			} else if vmim.Status.MigrationState.AbortStatus == virtv1.MigrationAbortFailed {
				return "Migration canceled failed", nil
			} else if vmim.Status.MigrationState.AbortStatus == virtv1.MigrationAbortInProgress {
				return "Migration cancel in progress", nil
			}
			if vmim.Status.MigrationState.Completed {
				return "", nil
			}
		}
	}

	vmi := &virtv1.VirtualMachineInstance{}
	if err := client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: namespace, Name: vmName}, vmi); err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Sprintf("VMI %s not found in namespace %s", vmName, namespace), nil
		} else {
			return err.Error(), err
		}
	}
	volumeChange := false
	liveMigrateable := false
	liveMigrateableMessage := ""
	for _, condition := range vmi.Status.Conditions {
		if condition.Type == virtv1.VirtualMachineInstanceVolumesChange {
			volumeChange = condition.Status == corev1.ConditionTrue
		}
		if condition.Type == virtv1.VirtualMachineInstanceIsMigratable {
			liveMigrateable = condition.Status == corev1.ConditionTrue
			liveMigrateableMessage = condition.Message
		}
	}
	if volumeChange && !liveMigrateable {
		// Unable to storage live migrate because something else is preventing migration
		return liveMigrateableMessage, nil
	}
	return "", nil
}

func cancelLiveMigration(client k8sclient.Client, vmName, namespace string, volumes *vmVolumes, log logr.Logger) error {
	vm := &virtv1.VirtualMachine{}
	if err := client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: namespace, Name: vmName}, vm); err != nil {
		return err
	}

	log.V(3).Info("Canceling live migration", "vm", vmName)
	if err := updateVM(client, vm, volumes.targetVolumes, volumes.sourceVolumes, log); err != nil {
		return err
	}
	return nil
}

func liveMigrationsCompleted(client k8sclient.Client, namespace string, vmNames []string) (bool, error) {
	vmim := &virtv1.VirtualMachineInstanceMigrationList{}
	if err := client.List(context.TODO(), vmim, k8sclient.InNamespace(namespace)); err != nil {
		return false, err
	}
	completed := true
	for _, migration := range vmim.Items {
		if slices.Contains(vmNames, migration.Spec.VMIName) {
			if migration.Status.Phase != virtv1.MigrationSucceeded {
				completed = false
				break
			}
		}
	}
	return completed, nil
}

func updateVM(client k8sclient.Client, vm *virtv1.VirtualMachine, sourceVolumes, targetVolumes []string, log logr.Logger) error {
	if vm == nil || vm.Name == "" {
		return nil
	}
	vmCopy := vm.DeepCopy()
	// Ensure the VM migration strategy is set properly.
	log.V(5).Info("Setting volume migration strategy to migration", "vm", vmCopy.Name)
	vmCopy.Spec.UpdateVolumesStrategy = ptr.To[virtv1.UpdateVolumesStrategy](virtv1.UpdateVolumesStrategyMigration)

	for i := 0; i < len(sourceVolumes); i++ {
		// Check if we need to update DataVolumeTemplates.
		for j, dvTemplate := range vmCopy.Spec.DataVolumeTemplates {
			if dvTemplate.Name == sourceVolumes[i] {
				log.V(5).Info("Updating DataVolumeTemplate", "source", sourceVolumes[i], "target", targetVolumes[i])
				vmCopy.Spec.DataVolumeTemplates[j].Name = targetVolumes[i]
			}
		}
		for j, volume := range vm.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == sourceVolumes[i] {
				log.V(5).Info("Updating PersistentVolumeClaim", "source", sourceVolumes[i], "target", targetVolumes[i])
				vmCopy.Spec.Template.Spec.Volumes[j].PersistentVolumeClaim.ClaimName = targetVolumes[i]
			}
			if volume.DataVolume != nil && volume.DataVolume.Name == sourceVolumes[i] {
				log.V(5).Info("Updating DataVolume", "source", sourceVolumes[i], "target", targetVolumes[i])
				if err := CreateNewDataVolume(client, sourceVolumes[i], targetVolumes[i], vmCopy.Namespace, log); err != nil {
					return err
				}
				vmCopy.Spec.Template.Spec.Volumes[j].DataVolume.Name = targetVolumes[i]
			}
		}
	}
	if !reflect.DeepEqual(vm, vmCopy) {
		log.V(5).Info("Calling VM update", "vm", vm.Name)
		if err := client.Update(context.TODO(), vmCopy); err != nil {
			return err
		}
	} else {
		log.V(5).Info("No changes to VM", "vm", vm.Name)
	}
	return nil
}

func CreateNewDataVolume(client k8sclient.Client, sourceDvName, targetDvName, ns string, log logr.Logger) error {
	log.V(3).Info("Create new adopting datavolume from source datavolume", "namespace", ns, "source name", sourceDvName, "target name", targetDvName)
	originalDv := &cdiv1.DataVolume{}
	if err := client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: ns, Name: sourceDvName}, originalDv); err != nil {
		log.Error(err, "Failed to get source datavolume", "namespace", ns, "name", sourceDvName)
		return err
	}

	// Create adopting datavolume.
	adoptingDV := originalDv.DeepCopy()
	adoptingDV.Name = targetDvName
	if adoptingDV.Annotations == nil {
		adoptingDV.Annotations = make(map[string]string)
	}
	adoptingDV.Annotations["cdi.kubevirt.io/allowClaimAdoption"] = "true"
	adoptingDV.ResourceVersion = ""
	adoptingDV.ManagedFields = nil
	adoptingDV.UID = ""
	adoptingDV.Spec.Source = &cdiv1.DataVolumeSource{
		Blank: &cdiv1.DataVolumeBlankImage{},
	}
	if adoptingDV.Spec.Storage != nil {
		if _, ok := adoptingDV.Spec.Storage.Resources.Requests[corev1.ResourceStorage]; !ok {
			// DV doesn't have a size specified, look it up from the matching PVC.
			pvc := &corev1.PersistentVolumeClaim{}
			if err := client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: ns, Name: sourceDvName}, pvc); err != nil {
				return err
			}
			adoptingDV.Spec.Storage.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
			}
			adoptingDV.Spec.Storage.Resources.Requests[corev1.ResourceStorage] = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		}
	}
	err := client.Create(context.TODO(), adoptingDV)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		log.Error(err, "Failed to create adopting datavolume", "namespace", ns, "name", targetDvName)
		return err
	}
	return nil
}

// Gets all the VirtualMachineInstanceMigration objects by volume in the passed in namespace.
func (t *Task) getVolumeVMIMInNamespaces(namespaces []string) (map[string]*virtv1.VirtualMachineInstanceMigration, error) {
	vmimMap := make(map[string]*virtv1.VirtualMachineInstanceMigration)
	for _, namespace := range namespaces {
		volumeVMMap, err := getRunningVmVolumeMap(t.sourceClient, namespace)
		if err != nil {
			return nil, err
		}
		for volumeName, vmName := range volumeVMMap {
			vmimList := &virtv1.VirtualMachineInstanceMigrationList{}
			if err := t.sourceClient.List(context.TODO(), vmimList, k8sclient.InNamespace(namespace)); err != nil {
				return nil, err
			}
			for _, vmim := range vmimList.Items {
				if vmim.Spec.VMIName == vmName {
					vmimMap[fmt.Sprintf("%s/%s", namespace, volumeName)] = &vmim
				}
			}
		}
	}
	return vmimMap, nil
}

func getVMIMElapsedTime(vmim *virtv1.VirtualMachineInstanceMigration) metav1.Duration {
	if vmim == nil || vmim.Status.MigrationState == nil {
		return metav1.Duration{
			Duration: 0,
		}
	}
	if vmim.Status.MigrationState.StartTimestamp == nil {
		for _, timestamps := range vmim.Status.PhaseTransitionTimestamps {
			if timestamps.Phase == virtv1.MigrationRunning {
				vmim.Status.MigrationState.StartTimestamp = &timestamps.PhaseTransitionTimestamp
			}
		}
	}
	if vmim.Status.MigrationState.StartTimestamp == nil {
		return metav1.Duration{
			Duration: 0,
		}
	}
	if vmim.Status.MigrationState.EndTimestamp != nil {
		return metav1.Duration{
			Duration: vmim.Status.MigrationState.EndTimestamp.Sub(vmim.Status.MigrationState.StartTimestamp.Time),
		}
	}
	return metav1.Duration{
		Duration: time.Since(vmim.Status.MigrationState.StartTimestamp.Time),
	}
}

func (t *Task) getLastObservedProgressPercent(vmName, namespace string, currentProgress map[string]*migapi.LiveMigrationProgress) (string, error) {
	if err := t.buildPrometheusAPI(); err != nil {
		return "", err
	}

	result, warnings, err := t.PromQuery(context.TODO(), fmt.Sprintf(progressQuery, vmName, vmName, vmName), time.Now())
	if err != nil {
		t.Log.Error(err, "Failed to query prometheus, returning previous progress")
		if progress, found := currentProgress[fmt.Sprintf("%s/%s", namespace, vmName)]; found {
			return progress.LastObservedProgressPercent, nil
		}
		return "", nil
	}
	if len(warnings) > 0 {
		t.Log.Info("Warnings", "warnings", warnings)
	}
	t.Log.V(5).Info("Prometheus query result", "type", result.Type(), "value", result.String())
	progress := parseProgress(result.String())
	if progress != "" {
		return progress + "%", nil
	}
	return "", nil
}

func (t *Task) buildPrometheusAPI() error {
	if t.PrometheusAPI != nil {
		return nil
	}
	if t.restConfig == nil {
		var err error
		t.restConfig, err = t.PlanResources.SrcMigCluster.BuildRestConfig(t.Client)
		if err != nil {
			return err
		}
	}
	url, err := t.buildSourcePrometheusEndPointURL()
	if err != nil {
		return err
	}

	// Prometheus URL not found, return blank progress
	if url == "" {
		return nil
	}
	httpClient, err := rest.HTTPClientFor(t.restConfig)
	if err != nil {
		return err
	}
	client, err := prometheusapi.NewClient(prometheusapi.Config{
		Address: url,
		Client:  httpClient,
	})
	if err != nil {
		return err
	}
	t.PrometheusAPI = prometheusv1.NewAPI(client)
	t.PromQuery = t.PrometheusAPI.Query
	return nil
}

func parseProgress(progress string) string {
	regExp := regexp.MustCompile(`\=\> (\d{1,3})\.\d* @`)

	if regExp.MatchString(progress) {
		return regExp.FindStringSubmatch(progress)[1]
	}
	return ""
}

// Find the URL that contains the prometheus metrics on the source cluster.
func (t *Task) buildSourcePrometheusEndPointURL() (string, error) {
	urlString, err := t.getPrometheusURLFromConfig()
	if err != nil {
		return "", err
	}
	if urlString == "" {
		// URL not found in config map, attempt to get the open shift prometheus route.
		routes := &routev1.RouteList{}
		req, err := labels.NewRequirement("app.kubernetes.io/part-of", selection.Equals, []string{"openshift-monitoring"})
		if err != nil {
			return "", err
		}
		if err := t.sourceClient.List(context.TODO(), routes, k8sclient.InNamespace("openshift-monitoring"), &k8sclient.ListOptions{
			LabelSelector: labels.NewSelector().Add(*req),
		}); err != nil {
			return "", err
		}
		for _, r := range routes.Items {
			if r.Spec.To.Name == prometheusRoute {
				urlString = r.Spec.Host
				break
			}
		}
	}
	if urlString == "" {
		// Don't return error just return empty and skip the progress report
		return "", nil
	}
	parsedUrl, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}
	parsedUrl.Scheme = "https"
	urlString = parsedUrl.String()
	t.Log.V(3).Info("Prometheus route URL", "url", urlString)
	return urlString, nil
}

// The key in the config map should be in format <source cluster name>_PROMETHEUS_URL
// For instance if the cluster name is "cluster1" the key should be "cluster1_PROMETHEUS_URL"
func (t *Task) getPrometheusURLFromConfig() (string, error) {
	migControllerConfigMap := &corev1.ConfigMap{}
	if err := t.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: migapi.OpenshiftMigrationNamespace, Name: "migration-controller"}, migControllerConfigMap); err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s_%s", t.PlanResources.SrcMigCluster.Name, prometheusURLKey)
	if prometheusURL, found := migControllerConfigMap.Data[key]; found {
		return prometheusURL, nil
	}
	return "", nil
}

func (t *Task) deleteStaleVirtualMachineInstanceMigrations() error {
	pvcMap, err := t.getNamespacedPVCPairs()
	if err != nil {
		return err
	}

	for namespacePair := range pvcMap {
		namespace, err := getNamespace(namespacePair)
		if err != nil && !errors.Is(err, ErrNamespacesDoNotMatch) {
			return err
		} else if errors.Is(err, ErrNamespacesDoNotMatch) {
			continue
		}
		vmMap, err := getVMNamesInNamespace(t.sourceClient, namespace)
		if err != nil {
			return err
		}

		vmimList := &virtv1.VirtualMachineInstanceMigrationList{}
		if err := t.sourceClient.List(context.TODO(), vmimList, k8sclient.InNamespace(namespace)); err != nil {
			if meta.IsNoMatchError(err) {
				return nil
			}
			return err
		}
		for _, vmim := range vmimList.Items {
			if _, ok := vmMap[vmim.Spec.VMIName]; ok {
				if vmim.Status.Phase == virtv1.MigrationSucceeded &&
					vmim.Status.MigrationState != nil &&
					vmim.Status.MigrationState.EndTimestamp.Before(&t.Owner.CreationTimestamp) {
					// Old VMIM that has completed and succeeded before the migration was created, delete the VMIM
					if err := t.sourceClient.Delete(context.TODO(), &vmim); err != nil && !k8serrors.IsNotFound(err) {
						return err
					}
				}
			}
		}
	}
	return nil
}
