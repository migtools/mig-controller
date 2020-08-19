package migmigration

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
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
		log.Trace(err)
		return counter, err
	}

	for _, stagePod := range stagePods {
		if existingPods.contains(stagePod) {
			continue
		}
		err := client.Create(context.TODO(), &stagePod.Pod)
		if err != nil && !k8serr.IsAlreadyExists(err) {
			return 0, err
		}
		counter++
	}

	return counter + len(existingPods), nil
}

func (t *Task) listStagePods(client k8sclient.Client) (StagePodList, error) {
	podList := corev1.PodList{}
	options := k8sclient.MatchingLabels(t.stagePodLabels())
	err := client.List(context.TODO(), options, &podList)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	return BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podList.Items, stagePodImage, resourceLimitMapping), nil
}

func (t *Task) getStagePodImage(client k8sclient.Client) (string, error) {
	clusterConfig := &corev1.ConfigMap{}
	clusterConfigRef := types.NamespacedName{Name: migapi.ClusterConfigMapName, Namespace: migapi.VeleroNamespace}
	err := client.Get(context.TODO(), clusterConfigRef, clusterConfig)
	if err != nil {
		log.Trace(err)
		return "", err
	}
	stagePodImage, ok := clusterConfig.Data[migapi.StagePodImageKey]
	if !ok {
		err := errors.Errorf("configmap key not found: %v", migapi.StagePodImageKey)
		log.Trace(err)
		return "", err
	}
	return stagePodImage, nil
}

func (t *Task) ensureStagePodsFromOrphanedPVCs() error {
	stagePods := StagePodList{}
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}

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

	pvcMapping := t.getPVCs()

	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		log.Trace(err)
		return err
	}
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		log.Trace(err)
		return err
	}

	for _, ns := range t.sourceNamespaces() {
		list := &corev1.PersistentVolumeClaimList{}
		err = client.List(context.TODO(), k8sclient.InNamespace(ns), list)
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

	created, err := t.createStagePods(client, stagePods)
	if err != nil {
		log.Trace(err)
		return err
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
		log.Trace(err)
		return err
	}

	podTemplates, err := migpods.ListTemplatePods(client, t.sourceNamespaces())
	if err != nil {
		log.Trace(err)
		return err
	}

	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		log.Trace(err)
		return err
	}
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	stagePods := BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podTemplates, stagePodImage, resourceLimitMapping)

	created, err := t.createStagePods(client, stagePods)
	if err != nil {
		log.Trace(err)
		return err
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
		err := client.List(context.TODO(), k8sclient.InNamespace(ns), &limitRangeList)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		memVal, err := resource.ParseQuantity(defaultMemory)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		cpuVal, err := resource.ParseQuantity(defaultCPU)
		if err != nil {
			log.Trace(err)
			return nil, err
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
		log.Trace(err)
		return err
	}
	stagePods := StagePodList{}
	resourceLimitMapping, err := buildResourceLimitMapping(t.sourceNamespaces(), client)
	if err != nil {
		log.Trace(err)
		return err
	}
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, ns := range t.sourceNamespaces() {
		podList := corev1.PodList{}
		err := client.List(context.TODO(), k8sclient.InNamespace(ns), &podList)
		if err != nil {
			log.Trace(err)
			return err
		}
		stagePods.merge(BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podList.Items, stagePodImage, resourceLimitMapping)...)
	}

	created, err := t.createStagePods(client, stagePods)
	if err != nil {
		log.Trace(err)
		return err
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

// Ensure the stage pods are Running.
func (t *Task) ensureStagePodsStarted() (report PodStartReport, err error) {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return
	}
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
					log.Trace(err)
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
	options := k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels())
	err = client.List(context.TODO(), options, &podList)
	if err != nil {
		return
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			if !hasHealthyClaims(&pod) {
				report.failed = true
			}
			return
		}
	}

	report.started = true
	return
}

// Ensure the stage pods have been deleted.
func (t *Task) ensureStagePodsDeleted() error {
	clients, err := t.getBothClients()
	if err != nil {
		log.Trace(err)
		return err
	}
	options := k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels())
	for _, client := range clients {
		podList := corev1.PodList{}
		err := client.List(context.TODO(), options, &podList)
		if err != nil {
			return err
		}
		for _, pod := range podList.Items {
			// Delete
			err := client.Delete(context.TODO(), &pod)
			if err != nil && !k8serr.IsNotFound(err) {
				log.Trace(err)
				return err
			}
			log.Info(
				"Stage pod deleted.",
				"ns",
				pod.Namespace,
				"name",
				pod.Name)
		}
	}

	return nil
}

// Ensure the deleted stage pods have finished terminating
func (t *Task) ensureStagePodsTerminated() (bool, error) {
	clients, err := t.getBothClients()
	if err != nil {
		log.Trace(err)
		return false, err
	}

	terminatedPhases := map[corev1.PodPhase]bool{
		corev1.PodSucceeded: true,
		corev1.PodFailed:    true,
		corev1.PodUnknown:   true,
	}
	options := k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels())
	for _, client := range clients {
		podList := corev1.PodList{}
		err := client.List(context.TODO(), options, &podList)
		if err != nil {
			return false, err
		}
		for _, pod := range podList.Items {
			// looks like it's terminated
			if terminatedPhases[pod.Status.Phase] {
				continue
			}
			return false, nil
		}
	}
	t.Owner.Status.DeleteCondition(StagePodsCreated)
	return true, nil
}

func (t *Task) stagePodLabels() map[string]string {
	labels := t.Owner.GetCorrelationLabels()
	labels[IncludedInStageBackupLabel] = t.UID()

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
