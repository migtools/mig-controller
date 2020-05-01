package migmigration

import (
	"context"
	"reflect"
	"strconv"

	"k8s.io/apimachinery/pkg/api/errors"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// StagePod - wrapper for stage pod, allowing to compare  two stage pods for equality
type StagePod struct {
	corev1.Pod
}

// StagePodList - a list of stage pods, with built-in stage pod deduplication
type StagePodList []StagePod

// BuildStagePods - creates a list of stage pods from a list of pods
func BuildStagePods(labels map[string]string, pvcMapping map[k8sclient.ObjectKey]migapi.PV, list *[]corev1.Pod) StagePodList {
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
		stagePod := buildStagePodFromPod(migref.ObjectKey(&pod), labels, &pod, volumes)
		if stagePod != nil {
			stagePods.merge(*stagePod)
		}
	}
	return stagePods
}

func (p StagePod) volumesContained(pod StagePod) bool {
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
		if err != nil && !errors.IsAlreadyExists(err) {
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
	return BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podList.Items), nil
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
			claimKey := migref.ObjectKey(&pvc)
			pv, found := pvcMapping[claimKey]
			if !found ||
				pv.Selection.Action != migapi.PvCopyAction ||
				pv.Selection.CopyMethod != migapi.PvFilesystemCopyMethod {
				continue
			}
			if pvcMounted(existingStagePods, claimKey) {
				continue
			}
			stagePods.merge(*buildStagePod(pvc, t.stagePodLabels()))
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

	podTemplates, err := t.PlanResources.MigPlan.ListTemplatePods(client)
	if err != nil {
		log.Trace(err)
		return err
	}

	stagePods := BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podTemplates)

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
func (t *Task) ensureStagePodsFromRunning() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	stagePods := StagePodList{}
	for _, ns := range t.sourceNamespaces() {
		podList := corev1.PodList{}
		err := client.List(context.TODO(), k8sclient.InNamespace(ns), &podList)
		if err != nil {
			log.Trace(err)
			return err
		}
		stagePods.merge(BuildStagePods(t.stagePodLabels(), t.getPVCs(), &podList.Items)...)
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
func (t *Task) ensureStagePodsStarted() (bool, error) {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return false, err
	}

	podList := corev1.PodList{}
	options := k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels())
	err = client.List(context.TODO(), options, &podList)
	if err != nil {
		return false, err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}
	}

	return true, nil
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
			if err != nil && !errors.IsNotFound(err) {
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

// Build a stage pod based on existing pod.
func buildStagePodFromPod(ref k8sclient.ObjectKey, labels map[string]string, pod *corev1.Pod, pvcVolumes []corev1.Volume) *StagePod {
	// Base pod.
	newPod := &StagePod{
		Pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ref.Namespace,
				GenerateName: migref.TruncateName("stage-"+ref.Name) + "-",
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
			Image:        "registry.access.redhat.com/rhel7",
			Command:      []string{"sleep"},
			Args:         []string{"infinity"},
			VolumeMounts: volumeMounts,
		}
		newPod.Spec.Containers = append(newPod.Spec.Containers, stageContainer)
	}

	return newPod
}

// Build a generic stage pod for PVC, where no pod template could be used.
func buildStagePod(pvc corev1.PersistentVolumeClaim, labels map[string]string) *StagePod {
	// Base pod.
	newPod := &StagePod{
		Pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    pvc.Namespace,
				GenerateName: migref.TruncateName("stage-"+pvc.Name) + "-",
				Labels:       labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:    "sleep",
					Image:   "registry.access.redhat.com/rhel7",
					Command: []string{"sleep"},
					Args:    []string{"infinity"},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "stage",
						MountPath: "/var/data",
					}},
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
