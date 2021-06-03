package migmigration

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migevent "github.com/konveyor/mig-controller/pkg/event"
	migpods "github.com/konveyor/mig-controller/pkg/pods"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// StagePod - wrapper for stage pod, allowing to compare  two stage pods for equality
type StagePod struct {
	corev1.Pod
}

// StagePodList - a list of stage pods, with built-in stage pod deduplication
type StagePodList []StagePod

// resource limit mapping elements
const (
	memory        = "memory"
	cpu           = "cpu"
	defaultMemory = "128Mi"
	defaultCPU    = "100m"
)

// Stage pod start report.
type PodStartReport struct {
	// failed detected.
	failed bool
	// pod failed reasons.
	reasons []string
	// all pods started.
	started bool
	// Progress of the stage pod
	progress []string
}

// BuildStagePods - creates a list of stage pods from a list of pods
func BuildStagePods(labels map[string]string,
	pvcMapping map[k8sclient.ObjectKey]migapi.PV,
	list *[]corev1.Pod, stagePodImage string,
	resourceLimitMapping map[string]map[string]resource.Quantity) StagePodList {

	stagePods := StagePodList{}
	for _, pod := range *list {
		volumes := []corev1.Volume{}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim == nil {
				continue
			}
			claimKey := k8sclient.ObjectKey{
				Name:      volume.PersistentVolumeClaim.ClaimName,
				Namespace: pod.Namespace,
			}
			pv, found := pvcMapping[claimKey]
			if !found ||
				pv.Selection.Action != migapi.PvCopyAction ||
				pv.Selection.CopyMethod != migapi.PvFilesystemCopyMethod {
				continue
			}
			volumes = append(volumes, volume)
		}
		if len(volumes) == 0 {
			continue
		}
		podKey := k8sclient.ObjectKey{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
		}
		stagePod := buildStagePodFromPod(podKey, labels, &pod, volumes, stagePodImage, resourceLimitMapping)
		if stagePod != nil {
			stagePods.merge(*stagePod)
		}
	}
	return stagePods
}

func (p StagePod) volumesContained(pod StagePod) bool {
	if p.Namespace != pod.Namespace {
		return false
	}
	for _, volume := range p.Spec.Volumes {
		found := false
		for _, targetVolume := range pod.Spec.Volumes {
			if reflect.DeepEqual(volume.VolumeSource, targetVolume.VolumeSource) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (l *StagePodList) contains(pod StagePod) bool {
	for _, srcPod := range *l {
		if pod.volumesContained(srcPod) {
			return true
		}
	}

	return false
}

func (l *StagePodList) merge(list ...StagePod) {
	for _, pod := range list {
		if !l.contains(pod) {
			*l = append(*l, pod)
		}
	}
}

func (t *Task) createStagePods(client k8sclient.Client, stagePods StagePodList) (int, error) {
	counter := 0
	existingPods, err := t.listStagePods(client)
	if err != nil {
		return counter, liberr.Wrap(err)
	}

	for _, stagePod := range stagePods {
		if existingPods.contains(stagePod) {
			t.Log.Info("Found existing Stage Pod, skipping",
				"pod", path.Join(stagePod.Namespace, stagePod.Name),
				"podPhase", stagePod.Status.Phase)
			continue
		}
		t.Log.Info("Creating Stage Pod.",
			"pod", path.Join(stagePod.Namespace, stagePod.Name))
		err := client.Create(context.TODO(), &stagePod.Pod)
		if err != nil && !k8serr.IsAlreadyExists(err) {
			return 0, liberr.Wrap(err)
		}
		counter++
	}

	return counter + len(existingPods), nil
}

func (t *Task) listStagePods(client k8sclient.Client) (StagePodList, error) {
	podList := corev1.PodList{}
	options := k8sclient.MatchingLabels(t.stagePodLabels())
	err := client.List(context.TODO(), &podList, options)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podList.Items, stagePodImage, resourceLimitMapping), nil
}

func (t *Task) getStagePodImage(client k8sclient.Client) (string, error) {
	clusterConfig := &corev1.ConfigMap{}
	clusterConfigRef := types.NamespacedName{Name: migapi.ClusterConfigMapName, Namespace: migapi.VeleroNamespace}
	err := client.Get(context.TODO(), clusterConfigRef, clusterConfig)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	stagePodImage, ok := clusterConfig.Data[migapi.StagePodImageKey]
	if !ok {
		return "", liberr.Wrap(errors.Errorf("Key [%v] not found in ConfigMap [%v/%v]",
			migapi.StagePodImageKey, clusterConfigRef.Namespace, clusterConfigRef.Name))
	}
	t.Log.Info("Got Stage Pod image from ConfigMap",
		"stagePodImage", stagePodImage,
		"configMap", path.Join(clusterConfigRef.Namespace, clusterConfigRef.Name))
	return stagePodImage, nil
}

func (t *Task) ensureStagePodsFromOrphanedPVCs() error {
	stagePods := StagePodList{}
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	t.Log.Info("Getting list of existing Stage Pods")
	existingStagePods, err := t.listStagePods(client)
	if err != nil {
		log.Trace(err)
		return nil
	}

	pvcMounted := func(list StagePodList, claimRef k8sclient.ObjectKey) bool {
		for _, pod := range list {
			if pod.Namespace != claimRef.Namespace {
				continue
			}
			for _, volume := range pod.Spec.Volumes {
				claim := volume.PersistentVolumeClaim
				if claim != nil && claim.ClaimName == claimRef.Name {
					return true
				}
			}
		}
		return false
	}

	t.Log.Info("Getting list of PVCs")
	pvcMapping := t.getPVCs()

	t.Log.Info("Building resource limit mapping for Stage Pods from Orphaned PVCs")
	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		return liberr.Wrap(err)
	}
	t.Log.Info("Getting Stage Pod image")
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, ns := range t.sourceNamespaces() {
		list := &corev1.PersistentVolumeClaimList{}
		err = client.List(context.TODO(), list, k8sclient.InNamespace(ns))
		if err != nil {
			log.Trace(err)
			return nil
		}
		for _, pvc := range list.Items {
			// Exclude unbound PVCs
			if pvc.Status.Phase != corev1.ClaimBound {
				continue
			}
			claimKey := k8sclient.ObjectKey{
				Name:      pvc.GetName(),
				Namespace: pvc.GetNamespace(),
			}
			pv, found := pvcMapping[claimKey]
			if !found ||
				pv.Selection.Action != migapi.PvCopyAction ||
				pv.Selection.CopyMethod != migapi.PvFilesystemCopyMethod {
				continue
			}
			if pvcMounted(existingStagePods, claimKey) {
				continue
			}
			stagePods.merge(*buildStagePod(pvc, t.stagePodLabels(), stagePodImage, resourceLimitMapping))
		}
	}

	t.Log.Info("Creating Stage Pods on source cluster from Orphaned PVCs")
	created, err := t.createStagePods(client, stagePods)
	if err != nil {
		return liberr.Wrap(err)
	}

	if created > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StagePodsCreated,
			Status:   True,
			Reason:   Created,
			Category: Advisory,
			Message:  "[] Stage pods created.",
			Items:    []string{strconv.Itoa(created)},
			Durable:  true,
		})
	}

	return nil
}

// Ensure all stage pods from running pods withing the application were created
func (t *Task) ensureStagePodsFromTemplates() error {
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	t.Log.Info("Getting list of Stage Pods to create for DeploymentConfigs, Deployments, " +
		"Daemonsets, ReplicaSets, CronJobs, Jobs on source cluster")
	podTemplates, err := migpods.ListTemplatePods(client, t.sourceNamespaces())
	if err != nil {
		return liberr.Wrap(err)
	}

	t.Log.Info("Building Stage Pod resource mappings for DeploymentConfigs, Deployments, " +
		"Daemonsets, ReplicaSets, CronJobs, Jobs on source cluster")
	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		return liberr.Wrap(err)
	}

	t.Log.Info("Getting Stage Pod image for source cluster")
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		return liberr.Wrap(err)
	}

	t.Log.Info("Building Stage Pod definitions for DeploymentConfigs, Deployments, " +
		"Daemonsets, ReplicaSets, CronJobs, Jobs on source cluster")
	stagePods := BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podTemplates, stagePodImage, resourceLimitMapping)

	t.Log.Info("Creating Stage Pods for DeploymentConfigs, Deployments, " +
		"Daemonsets, ReplicaSets, CronJobs, Jobs on source cluster")
	created, err := t.createStagePods(client, stagePods)
	if err != nil {
		return liberr.Wrap(err)
	}

	if created > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StagePodsCreated,
			Status:   True,
			Reason:   Created,
			Category: Advisory,
			Message:  "[] Stage pods created.",
			Items:    []string{strconv.Itoa(created)},
			Durable:  true,
		})
	}
	return nil
}

func buildResourceLimitMapping(namespaces []string, client k8sclient.Client) (map[string]map[string]resource.Quantity, error) {
	resourceLimitMapping := make(map[string]map[string]resource.Quantity)
	for _, ns := range namespaces {
		resourceLimitMapping[ns] = make(map[string]resource.Quantity)
		limitRangeList := corev1.LimitRangeList{}
		err := client.List(context.TODO(), &limitRangeList, k8sclient.InNamespace(ns))
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		memVal, err := resource.ParseQuantity(defaultMemory)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		cpuVal, err := resource.ParseQuantity(defaultCPU)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		for _, limitRange := range limitRangeList.Items {
			for _, limit := range limitRange.Spec.Limits {
				if limit.Type == corev1.LimitTypeContainer || limit.Type == corev1.LimitTypePod {
					minMem, found := limit.Min[corev1.ResourceMemory]
					if found && minMem.Cmp(memVal) > 0 {
						memVal = minMem
					}
					minCpu, found := limit.Min[corev1.ResourceCPU]
					if found && minCpu.Cmp(cpuVal) > 0 {
						cpuVal = minCpu
					}
				}
			}
		}
		resourceLimitMapping[ns][memory] = memVal
		resourceLimitMapping[ns][cpu] = cpuVal
	}
	return resourceLimitMapping, nil
}

func parseResourceLimitMapping(ns string, mapping map[string]map[string]resource.Quantity) (resource.Quantity, resource.Quantity) {
	memVal, _ := resource.ParseQuantity(defaultMemory)
	cpuVal, _ := resource.ParseQuantity(defaultCPU)
	if mapping[ns] != nil {
		mappingMem, found := mapping[ns][memory]
		if found {
			memVal = mappingMem
		}
		mappingCPU, found := mapping[ns][cpu]
		if found {
			cpuVal = mappingCPU
		}
	}
	return memVal, cpuVal
}

// Ensure all stage pods from running pods withing the application were created
func (t *Task) ensureStagePodsFromRunning() error {
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	stagePods := StagePodList{}
	t.Log.Info("Building resource limit mapping for stage pods")
	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		return liberr.Wrap(err)
	}
	t.Log.Info("Retrieving stage pod image")
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, ns := range t.sourceNamespaces() {
		t.Log.Info("Building list of stage pods for src cluster namespace", "namespace", ns)
		podList := corev1.PodList{}
		err := client.List(context.TODO(), &podList, k8sclient.InNamespace(ns))
		if err != nil {
			return liberr.Wrap(err)
		}
		stagePods.merge(BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podList.Items, stagePodImage, resourceLimitMapping)...)
	}

	t.Log.Info("Creating stage pods on source cluster")
	created, err := t.createStagePods(client, stagePods)
	if err != nil {
		return liberr.Wrap(err)
	}

	if created > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StagePodsCreated,
			Status:   True,
			Reason:   Created,
			Category: Advisory,
			Message:  "[] Stage pods created.",
			Items:    []string{strconv.Itoa(created)},
			Durable:  true,
		})
	}

	return nil
}

// Ensure the stage pods are Running on source cluster.
func (t *Task) ensureSourceStagePodsStarted() (report PodStartReport, err error) {
	client, err := t.getSourceClient()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	t.Log.Info("Checking health of Stage Pods on source cluster.")
	return t.stagePodReport(client)
}

// Ensure the stage pods are Running on destination cluster.
func (t *Task) ensureDestinationStagePodsStarted() (report PodStartReport, err error) {
	client, err := t.getDestinationClient()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	t.Log.Info("Checking health of Stage Pods on destination cluster.")
	return t.stagePodReport(client)
}

func (t *Task) stagePodReport(client k8sclient.Client) (report PodStartReport, err error) {
	hasHealthyClaims := func(pod *corev1.Pod) (healthy bool) {
		healthy = true
		for _, vol := range pod.Spec.Volumes {
			claim := vol.PersistentVolumeClaim
			if claim == nil {
				continue
			}
			pvc := corev1.PersistentVolumeClaim{}
			key := k8sclient.ObjectKey{
				Namespace: pod.Namespace,
				Name:      claim.ClaimName,
			}
			err = client.Get(context.TODO(), key, &pvc)
			if err != nil {
				healthy = false
				if !k8serr.IsNotFound(err) {
					err = liberr.Wrap(err)
					return
				}
				report.reasons = append(
					report.reasons,
					fmt.Sprintf(
						"PVC: %s, not-found.",
						key))
				err = nil
				break
			}
			if pvc.DeletionTimestamp != nil {
				report.reasons = append(
					report.reasons,
					fmt.Sprintf(
						"PVC: %s, deleted.",
						key))
				healthy = false
				break
			}
			if pvc.Status.Phase != corev1.ClaimBound {
				report.reasons = append(
					report.reasons,
					fmt.Sprintf(
						"PVC: %s, not bound.",
						key))
				healthy = false
				break
			}
		}
		return
	}
	podList := corev1.PodList{}
	// Following line still uses MigMigration-based correlation labels since it is not part of rollback process
	options := k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels())
	err = client.List(context.TODO(), &podList, options)
	if err != nil {
		return
	}
	report.started = false
	for _, pod := range podList.Items {
		t.Log.V(4).Info("Checking if Stage Pod is healthy.",
			"pod", path.Join(pod.Namespace, pod.Name))

		// Logs abnormal events for Stage Pods if any are found
		migevent.LogAbnormalEventsForResource(
			client, t.Log,
			"Found abnormal event for Stage Pod",
			types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name},
			pod.UID, "Pod")

		initReady := true
		for _, c := range pod.Status.InitContainerStatuses {
			// If the init contianer is waiting, then nothing can happen.
			if c.State.Waiting != nil {
				// If the pod has unhealthy claims, we will fail the migration
				// So the user can fix the plan/pvc.
				if !hasHealthyClaims(&pod) {
					t.Log.Info("Found unhealthy PersistentVolumeClaims attached to Stage Pod container. ",
						"pod", path.Join(pod.Namespace, pod.Name),
						"container", c.Name)
					report.failed = true
					return
				}
				initReady = false
				t.Log.Info("Found Stage Pod container with "+
					"pod.Status.InitContainerStatuses.State.Waiting=true",
					"pod", path.Join(pod.Namespace, pod.Name),
					"container", c.Name)
				report.progress = append(
					report.progress,
					fmt.Sprintf(
						"Pod %s/%s: Container %s %s",
						pod.Namespace,
						pod.Name,
						c.Name,
						c.State.Waiting.Message))
			}
			if c.State.Terminated != nil && c.State.Terminated.ExitCode != 0 {
				t.Log.Info("Found unhealthy (terminated) Stage Pod container. "+
					"Init container failed with exit code %v.",
					"pod", path.Join(pod.Namespace, pod.Name),
					"container", c.Name,
					"containerExitCode", c.State.Terminated.ExitCode)
				initReady = false
				report.progress = append(
					report.progress,
					fmt.Sprintf(
						"Pod %s/%s: init container failed to finish",
						pod.Namespace,
						pod.Name))
			}
		}
		if !initReady {
			// If init container is not finished or started running warn the user.
			return
		}

		// if Pod is running, then move on
		// pod succeeded phase should never occur for a stage pod
		if pod.Status.Phase == corev1.PodRunning {
			t.Log.Info("Found healthy running Stage Pod.",
				"pod", path.Join(pod.Namespace, pod.Name))
			report.progress = append(
				report.progress,
				fmt.Sprintf(
					"Pod %s/%s: Running",
					pod.Namespace,
					pod.Name))
			report.started = true

			return
		}

		// handle pod pending status
		if pod.Status.Phase == corev1.PodPending {
			// If the pod has unhealthy claims, we will fail the migration
			// So the user can fix the plan/pvc.
			// pod Spec having the node name, means the pod has been scheduled
			// becuase the pod has been scheduled we know that the PVC should be bound
			t.Log.Info("Found pending Stage Pod.",
				"pod", path.Join(pod.Namespace, pod.Name))
			if pod.Spec.NodeName != "" && !hasHealthyClaims(&pod) {
				t.Log.Info("Pending Stage Pod has unhealthy "+
					"claims after being scheduled to node",
					"pod", path.Join(pod.Namespace, pod.Name),
					"node", pod.Spec.NodeName)
				report.failed = true
				return
			}
			for _, c := range pod.Status.ContainerStatuses {
				if c.State.Waiting != nil {
					t.Log.Info("Pending Stage Pod container is waiting with message.",
						"pod", path.Join(pod.Namespace, pod.Name),
						"container", c.Name,
						"containerWaitingMessage", c.State.Waiting.Message)
					report.progress = append(
						report.progress,
						fmt.Sprintf(
							"Pod %s/%s: Container %s %s",
							pod.Namespace,
							pod.Name,
							c.Name,
							c.State.Waiting.Message))
				}
			}
		}

		//TODO: [shurley] eventually, this should cause us to backoff and re-try to the create the pod.
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
			t.Log.Info("Found Stage Pod with Status.Phase.",
				"pod", path.Join(pod.Namespace, pod.Name),
				"podPhase", pod.Status.Phase)
			report.failed = true
			report.progress = append(report.progress, report.reasons...)
		}
	}
	return
}

// Match number of stage pods in source and destination cluster
func (t *Task) allStagePodsMatch() (report []string, err error) {
	t.Log.Info("Checking that Stage Pods match on source and destination clusters.")
	dstClient, err := t.getDestinationClient()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	podDList := corev1.PodList{}

	options := k8sclient.MatchingLabels(t.stagePodLabels())
	err = dstClient.List(context.TODO(), &podDList, options)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	srcClient, err := t.getSourceClient()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	podSList := corev1.PodList{}
	err = srcClient.List(context.TODO(), &podSList, options)

	dPods := make(map[string]string)

	for _, pod := range podDList.Items {
		dPods[pod.Name] = pod.Name
	}

	namespaces := make(map[string]bool)
	for _, srcNamespace := range t.PlanResources.MigPlan.GetSourceNamespaces() {
		namespaces[srcNamespace] = true
	}

	for _, pod := range podSList.Items {
		if _, exists := namespaces[pod.Namespace]; exists {
			if _, exist := dPods[pod.Name]; !exist {
				t.Log.Info("Missing Stage Pod on destination cluster. Migration may fail.",
					"pod", path.Join(pod.Namespace, pod.Name))
				report = append(report, pod.Name+" is missing. Migration might fail")
			}
		}
	}

	if len(report) > 0 {
		return
	}

	stageReport, err := t.ensureDestinationStagePodsStarted()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !stageReport.started {
		report = stageReport.progress
		return
	}
	report = []string{"All the stage pods are restored, waiting for restore to Complete"}
	return
}

// Ensure the stage pods have been deleted.
func (t *Task) ensureStagePodsDeleted() error {
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	destClient, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	t.Log.Info("Checking for leftover Stage Pods on source cluster")
	// Clean up source cluster namespaces
	for _, srcNamespace := range t.PlanResources.MigPlan.GetSourceNamespaces() {
		podList := corev1.PodList{}
		err := srcClient.List(
			context.TODO(),
			&podList,
			k8sclient.MatchingLabels(t.stagePodCleanupLabel()),
			k8sclient.InNamespace(srcNamespace),
		)
		if err != nil {
			return err
		}
		for _, pod := range podList.Items {
			t.Log.Info("Deleting Stage Pod on source cluster",
				"pod", path.Join(pod.Namespace, pod.Name))
			err := srcClient.Delete(context.TODO(), &pod)
			if err != nil && !k8serr.IsNotFound(err) {
				return liberr.Wrap(err)
			}
			log.Info("Stage Pod deletion requested on source cluster.",
				"pod", path.Join(pod.Namespace, pod.Name))
		}
	}

	t.Log.Info("Checking for leftover Stage Pods on destination cluster")
	// Clean up destination cluster namespaces
	for _, destNamespace := range t.PlanResources.MigPlan.GetDestinationNamespaces() {
		podList := corev1.PodList{}
		err := destClient.List(
			context.TODO(),
			&podList,
			k8sclient.MatchingLabels(t.stagePodCleanupLabel()),
			k8sclient.InNamespace(destNamespace),
		)
		if err != nil {
			return err
		}
		for _, pod := range podList.Items {
			t.Log.Info("Deleting Stage Pod on destination cluster",
				"pod", path.Join(pod.Namespace, pod.Name))
			err := destClient.Delete(context.TODO(), &pod)
			if err != nil && !k8serr.IsNotFound(err) {
				return liberr.Wrap(err)
			}
			log.Info("Stage Pod deletion requested on destination cluster.",
				"pod", path.Join(pod.Namespace, pod.Name))
		}
	}

	return nil
}

// Ensure the deleted stage pods have finished terminating
func (t *Task) ensureStagePodsTerminated() (bool, error) {
	srcClient, err := t.getSourceClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	destClient, err := t.getDestinationClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}

	terminatedPhases := map[corev1.PodPhase]bool{
		corev1.PodSucceeded: true,
		corev1.PodFailed:    true,
		corev1.PodUnknown:   true,
	}

	t.Log.Info("Checking if source cluster Stage Pods are terminated")
	for _, srcNamespace := range t.PlanResources.MigPlan.GetSourceNamespaces() {
		podList := corev1.PodList{}
		err := srcClient.List(
			context.TODO(),
			&podList,
			k8sclient.MatchingLabels(t.stagePodCleanupLabel()),
			k8sclient.InNamespace(srcNamespace),
		)
		if err != nil {
			return false, liberr.Wrap(err)
		}
		for _, pod := range podList.Items {
			// Check if Pod phase is one of 'terminatedPhases'
			if terminatedPhases[pod.Status.Phase] {
				continue
			}
			t.Log.Info("Found un-terminated Stage Pod in source cluster.",
				"pod", path.Join(pod.Namespace, pod.Name),
				"podPhase", pod.Status.Phase)
			return false, nil
		}
	}

	t.Log.Info("Checking if destination cluster Stage Pods are terminated")
	for _, destNamespace := range t.PlanResources.MigPlan.GetDestinationNamespaces() {
		podList := corev1.PodList{}
		err := destClient.List(
			context.TODO(),
			&podList,
			k8sclient.MatchingLabels(t.stagePodCleanupLabel()),
			k8sclient.InNamespace(destNamespace),
		)
		if err != nil {
			return false, liberr.Wrap(err)
		}
		for _, pod := range podList.Items {
			// Check if Pod phase is one of 'terminatedPhases'
			if terminatedPhases[pod.Status.Phase] {
				continue
			}
			t.Log.Info("Found un-terminated Stage Pod in destination cluster.",
				"pod", path.Join(pod.Namespace, pod.Name),
				"podPhase", pod.Status.Phase)
			return false, nil
		}
	}

	t.Owner.Status.DeleteCondition(StagePodsCreated)
	return true, nil
}

// Label applied to all stage pods for easy cleanup
func (t *Task) stagePodCleanupLabel() map[string]string {
	return map[string]string{migapi.StagePodLabel: migapi.True}
}

// Build map of all stage pod labels
// 1. MigPlan correlation label
// 2. MigMigration correlation label
// 3. IncludedInStageBackup label
// 4. StagePodIdentifier label
func (t *Task) stagePodLabels() map[string]string {
	labels := t.Owner.GetCorrelationLabels()
	migplanLabels := t.PlanResources.MigPlan.GetCorrelationLabels()

	// merge original migmigration correlation labels with migplan correlation label
	for labelName, labelValue := range migplanLabels {
		labels[labelName] = labelValue
	}

	labels[migapi.IncludedInStageBackupLabel] = t.UID()

	// merge label indicating this is a stage pod for later cleanup purposes
	stagePodCleanupLabel := t.stagePodCleanupLabel()
	for labelName, labelValue := range stagePodCleanupLabel {
		labels[labelName] = labelValue
	}

	return labels
}

func truncateName(name string) string {
	r := regexp.MustCompile(`(-+)`)
	name = r.ReplaceAllString(name, "-")
	name = strings.TrimRight(name, "-")
	if len(name) > 57 {
		name = name[:57]
	}
	return name
}

// Build a stage pod based on existing pod.
func buildStagePodFromPod(ref k8sclient.ObjectKey,
	labels map[string]string,
	pod *corev1.Pod,
	pvcVolumes []corev1.Volume, stagePodImage string,
	resourceLimitMapping map[string]map[string]resource.Quantity) *StagePod {

	podMemory, podCPU := parseResourceLimitMapping(ref.Namespace, resourceLimitMapping)
	// Base pod.
	newPod := &StagePod{
		Pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ref.Namespace,
				GenerateName: truncateName("stage-"+ref.Name) + "-",
				Labels:       labels,
			},
			Spec: corev1.PodSpec{
				Containers:                   []corev1.Container{},
				NodeName:                     pod.Spec.NodeName,
				Volumes:                      pvcVolumes,
				SecurityContext:              pod.Spec.SecurityContext,
				ServiceAccountName:           pod.Spec.ServiceAccountName,
				AutomountServiceAccountToken: pod.Spec.AutomountServiceAccountToken,
			},
		},
	}

	inVolumes := func(mount corev1.VolumeMount) bool {
		for _, volume := range pvcVolumes {
			if volume.Name == mount.Name {
				return true
			}
		}
		return false
	}

	// Add containers.
	for i, container := range pod.Spec.Containers {
		volumeMounts := []corev1.VolumeMount{}
		for _, mount := range container.VolumeMounts {
			if inVolumes(mount) {
				volumeMounts = append(volumeMounts, mount)
			}
		}
		stageContainer := corev1.Container{
			Name:         "sleep-" + strconv.Itoa(i),
			Image:        stagePodImage,
			Command:      []string{"sleep"},
			Args:         []string{"infinity"},
			VolumeMounts: volumeMounts,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					memory: podMemory,
					cpu:    podCPU,
				},
				Limits: corev1.ResourceList{
					memory: podMemory,
					cpu:    podCPU,
				},
			},
		}

		newPod.Spec.Containers = append(newPod.Spec.Containers, stageContainer)
	}

	return newPod
}

// Build a generic stage pod for PVC, where no pod template could be used.
func buildStagePod(pvc corev1.PersistentVolumeClaim,
	labels map[string]string, stagePodImage string,
	resourceLimitMapping map[string]map[string]resource.Quantity) *StagePod {

	podMemory, podCPU := parseResourceLimitMapping(pvc.Namespace, resourceLimitMapping)
	// Base pod.
	newPod := &StagePod{
		Pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    pvc.Namespace,
				GenerateName: truncateName("stage-"+pvc.Name) + "-",
				Labels:       labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:    "sleep",
					Image:   stagePodImage,
					Command: []string{"sleep"},
					Args:    []string{"infinity"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "stage",
						MountPath: "/var/data",
					}},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							memory: podMemory,
							cpu:    podCPU,
						},
						Limits: corev1.ResourceList{
							memory: podMemory,
							cpu:    podCPU,
						},
					},
				}},
				Volumes: []corev1.Volume{{
					Name: "stage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						}},
				}},
			},
		},
	}

	return newPod
}
