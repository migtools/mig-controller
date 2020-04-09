package migmigration

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var DefaultTemplates = []migapi.TemplateResource{
	{
		Resource:     "cronjob.batch",
		TemplatePath: ".spec.jobTemplate.spec.template",
	},
	{
		Resource:     "deployment.apps",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "deploymentconfig.apps.openshift.io",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "replicationcontroller",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "daemonset.apps",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "statefulset.apps",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "replicaset.apps",
		TemplatePath: ".spec.template",
	},
}

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
			volumes = append(volumes, vol.String())
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

func (l *StagePodList) create(client k8sclient.Client) error {
	for _, template := range *l {
		err := client.Create(context.TODO(), &template.Pod)
		if err != nil {
			return err
		}
	}

	return nil
}

// Build a stage pod based on `template`.
func (t *Task) buildStagePod(ref k8sclient.ObjectKey, podSpec *corev1.PodTemplateSpec) StagePod {
	labels := t.Owner.GetCorrelationLabels()
	labels[IncludedInStageBackupLabel] = t.UID()

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

// Ensure the stage pods have been created.
func (t *Task) ensureStagePodsCreated() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}

	templates := &StagePodList{}

	fromRunning, err := t.ensureStagePodsFromRunning(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	templates.merge(fromRunning...)

	podTemplates := append(DefaultTemplates, t.PlanResources.MigPlan.Spec.Templates...)
	fromTemplates, err := t.ensureStagePodsFromTemplates(client, podTemplates...)
	if err != nil {
		log.Trace(err)
		return err
	}
	templates.merge(fromTemplates...)

	err = templates.create(client)
	if err != nil {
		log.Trace(err)
		return err
	}

	if len(*templates) > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StagePodsCreated,
			Status:   True,
			Reason:   Created,
			Category: Advisory,
			Message:  "[] Stage pods created.",
			Items:    []string{strconv.Itoa(len(*templates))},
			Durable:  true,
		})
	}

	return nil
}

// Ensure all stage pods from running pods withing the application were created
func (t *Task) ensureStagePodsFromTemplates(client k8sclient.Client, templates ...migapi.TemplateResource) (StagePodList, error) {
	stagePods := StagePodList{}
	for _, template := range templates {
		// Get resources
		list := unstructured.UnstructuredList{}
		templateGVK := template.GroupKind().WithVersion("")
		list.SetGroupVersionKind(templateGVK)
		err := client.List(context.TODO(), &k8sclient.ListOptions{}, &list)
		if err != nil {
			return nil, err
		}
		resources := []unstructured.Unstructured{}
		for _, resource := range list.Items {
			for _, ns := range t.sourceNamespaces() {
				if resource.GetNamespace() == ns {
					resources = append(resources, resource)
					break
				}
			}
		}

		for _, resource := range resources {
			podTemplate := corev1.PodTemplateSpec{}
			unstructuredTemplate, found, err := unstructured.NestedMap(resource.Object, template.Path()...)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, fmt.Errorf("Template path %s was not found on resource: %s", template.TemplatePath, template.Resource)
			}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredTemplate, &podTemplate)
			if err != nil {
				return nil, fmt.Errorf("Unable to convert resource filed '%s' to 'PodTemplateSpec': %w", template.TemplatePath, err)
			}
			stagePods.merge(t.buildStagePod(migref.ObjectKey(&resource), &podTemplate))
		}
	}

	return stagePods, nil
}

// Ensure all stage pods from running pods withing the application were created
func (t *Task) ensureStagePodsFromRunning(client k8sclient.Client) (StagePodList, error) {
	stagePods := StagePodList{}
	for _, ns := range t.sourceNamespaces() {
		podList := corev1.PodList{}
		err := client.List(context.TODO(), k8sclient.InNamespace(ns), &podList)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		cLabel, _ := t.Owner.GetCorrelationLabel()
		for _, pod := range podList.Items {
			// Skip already created stage pods.
			if _, found := pod.Labels[cLabel]; found {
				continue
			}
			template := corev1.PodTemplateSpec{
				Spec: pod.Spec,
			}
			stagePods.merge(t.buildStagePod(migref.ObjectKey(&pod), &template))
		}
	}

	return stagePods, nil
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
