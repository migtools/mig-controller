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
func BuildStagePods(list *[]corev1.Pod) StagePodList {
	stagePods := StagePodList{}
	for _, pod := range *list {
		if pod.Spec.Volumes != nil {
			stagePods.merge(buildStagePodFromPod(migref.ObjectKey(&pod), pod.Labels, &pod))
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

func (l *StagePodList) create(client k8sclient.Client) (int, error) {
	existingPods := StagePodList{}
	if len(*l) > 0 {
		existingPods.list(client, (*l)[0].Labels)
	}
	counter := 0
	for _, stagePod := range *l {
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

func (l *StagePodList) list(client k8sclient.Client, labels map[string]string) error {
	podList := corev1.PodList{}
	options := k8sclient.MatchingLabels(labels)
	err := client.List(context.TODO(), options, &podList)
	if err != nil {
		return err
	}
	l.merge(BuildStagePods(&podList.Items)...)

	return nil
}

func (t *Task) ensureStagePodsFromOrphanedPVCs() error {
	existingStagePods := StagePodList{}
	stagePods := StagePodList{}
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}

	err = existingStagePods.list(client, t.stagePodLabels())
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
			if pvcMounted(existingStagePods, migref.ObjectKey(&pvc)) {
				continue
			}
			stagePods.merge(buildStagePod(pvc, t.stagePodLabels()))
		}
	}

	created, err := stagePods.create(client)
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

	stagePods := BuildStagePods(&podTemplates)

	created, err := stagePods.create(client)
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
		stagePods.merge(BuildStagePods(&podList.Items)...)
	}

	created, err := stagePods.create(client)
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
func buildStagePodFromPod(ref k8sclient.ObjectKey, labels map[string]string, pod *corev1.Pod) StagePod {
	// Base pod.
	newPod := StagePod{
		Pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ref.Namespace,
				Name:      "stage-" + ref.Name,
				Labels:    labels,
			},
			Spec: corev1.PodSpec{
				Containers:                   []corev1.Container{},
				NodeName:                     pod.Spec.NodeName,
				Volumes:                      pod.Spec.Volumes,
				SecurityContext:              pod.Spec.SecurityContext,
				ServiceAccountName:           pod.Spec.ServiceAccountName,
				AutomountServiceAccountToken: pod.Spec.AutomountServiceAccountToken,
			},
		},
	}
	// Add containers.
	for i, container := range pod.Spec.Containers {
		stageContainer := corev1.Container{
			Name:         "sleep-" + strconv.Itoa(i),
			Image:        "registry.access.redhat.com/rhel7",
			Command:      []string{"sleep"},
			Args:         []string{"infinity"},
			VolumeMounts: container.VolumeMounts,
		}
		newPod.Spec.Containers = append(newPod.Spec.Containers, stageContainer)
	}

	return newPod
}

// Build a generic stage pod for PVC, where no pod template could be used.
func buildStagePod(pvc corev1.PersistentVolumeClaim, labels map[string]string) StagePod {
	// Base pod.
	newPod := StagePod{
		Pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pvc.Namespace,
				Name:      "stage-" + string(pvc.UID),
				Labels:    labels,
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
