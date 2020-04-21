package migmigration

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type StagePod struct {
	corev1.Pod
}

type StagePodList []StagePod

func (p StagePod) equals(pod StagePod) bool {
	if len(p.Spec.Volumes) != len(pod.Spec.Volumes) {
		return false
	}

	volumeList := func(list []corev1.Volume) (volumes []string) {
		for _, vol := range list {
			volumes = append(volumes, vol.VolumeSource.String())
		}
		sort.Strings(volumes)
		return
	}

	targetVolumes := volumeList(pod.Spec.Volumes)
	for i, volume := range volumeList(p.Spec.Volumes) {
		if volume != targetVolumes[i] {
			return false
		}
	}

	return true
}

func (l *StagePodList) contains(pod StagePod) bool {
	for _, srcPod := range *l {
		if pod.equals(srcPod) {
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
	existingPods := &StagePodList{}
	if len(*l) > 0 {
		existingPods.list(client, (*l)[0].Labels)
	}
	counter := 0
	for _, stagePod := range *l {
		if existingPods.contains(stagePod) {
			continue
		}
		err := client.Create(context.TODO(), &stagePod.Pod)
		if err != nil {
			return 0, err
		}
		counter++
	}

	return counter + len(*existingPods), nil
}

func (l *StagePodList) list(client k8sclient.Client, labels map[string]string) error {
	podList := corev1.PodList{}
	options := k8sclient.MatchingLabels(labels)
	err := client.List(context.TODO(), options, &podList)
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		l.merge(buildStagePodFromPod(migref.ObjectKey(&pod), pod.Labels, &pod))
	}

	return nil
}

// Ensure all stage pods from running pods withing the application were created
func (t *Task) ensureStagePodsFromTemplates() error {
	stagePods := StagePodList{}
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}

	podTemplates, err := t.PlanResources.MigPlan.ListTemplates(client)
	for _, podTemplate := range podTemplates {
		stagePods.merge(buildStagePodFromTemplate(migref.ObjectKey(&podTemplate), t.stagePodLabels(), &podTemplate))
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
		cLabel, _ := t.Owner.GetCorrelationLabel()
		for _, pod := range podList.Items {
			// Skip already created stage pods.
			if _, found := pod.Labels[cLabel]; found {
				continue
			}
			stagePods.merge(buildStagePodFromPod(migref.ObjectKey(&pod), t.stagePodLabels(), &pod))
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
	template := &corev1.PodTemplateSpec{
		Spec: pod.Spec,
	}
	return buildStagePodFromTemplate(ref, labels, template)
}

// Build a stage pod based on `template`.
func buildStagePodFromTemplate(ref k8sclient.ObjectKey, labels map[string]string, podSpec *corev1.PodTemplateSpec) StagePod {
	// Map of Restic volumes.
	resticVolumes := map[string]bool{}
	for _, name := range strings.Split(podSpec.Annotations[ResticPvBackupAnnotation], ",") {
		resticVolumes[name] = true
	}
	// Base pod.
	newPod := StagePod{
		Pod: corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ref.Namespace,
				Name:      ref.Name + "-" + "stage",
				Labels:    labels,
			},
			Spec: corev1.PodSpec{
				Containers:                   []corev1.Container{},
				NodeName:                     podSpec.Spec.NodeName,
				Volumes:                      podSpec.Spec.Volumes,
				SecurityContext:              podSpec.Spec.SecurityContext,
				ServiceAccountName:           podSpec.Spec.ServiceAccountName,
				AutomountServiceAccountToken: podSpec.Spec.AutomountServiceAccountToken,
			},
		},
	}
	// Add containers.
	for i, container := range podSpec.Spec.Containers {
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
