package migmigration

import (
	"context"
	"strconv"
	"strings"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	pvdr "github.com/konveyor/mig-controller/pkg/cloudprovider"
	"github.com/konveyor/mig-controller/pkg/pods"
	"github.com/konveyor/mig-controller/pkg/reference"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/exec"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Delete the running restic pods.
// Restarted to get around mount propagation requirements.
func (t *Task) restartResticPods() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	list := corev1.PodList{}
	selector := labels.SelectorFromSet(map[string]string{
		"name": "restic",
	})
	err = client.List(
		context.TODO(),
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: selector,
		},
		&list)
	if err != nil {
		log.Trace(err)
		return err
	}

	for _, pod := range list.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		err = client.Delete(
			context.TODO(),
			&pod)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

// Determine if restic pod is running.
func (t *Task) haveResticPodsStarted() (bool, error) {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return false, err
	}

	discovery, err := t.getSourceDiscovery()
	if err != nil {
		log.Trace(err)
		return false, err
	}

	list := corev1.PodList{}
	selector := labels.SelectorFromSet(map[string]string{
		"name": "restic",
	})
	err = client.List(
		context.TODO(),
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: selector,
		},
		&list)
	if err != nil {
		log.Trace(err)
		return false, err
	}

	ds := &v1.DaemonSet{}
	kubeVersion, err := reference.GetKubeVersion(discovery)
	if err != nil {
		log.Trace(err)
		return false, err
	}
	if kubeVersion >= reference.AppsGap {
		err = client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      "restic",
				Namespace: migapi.VeleroNamespace,
			},
			ds)
	} else {
		dsOld := &v1beta1.DaemonSet{}
		err = client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      "restic",
				Namespace: migapi.VeleroNamespace,
			},
			dsOld)
		err = t.scheme.Convert(dsOld, ds, nil)
		if err != nil {
			log.Trace(err)
			return false, err
		}
	}

	if err != nil {
		log.Trace(err)
		return false, err
	}

	for _, pod := range list.Items {
		if pod.DeletionTimestamp != nil {
			return false, nil
		}
		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}
	}

	if ds.Status.CurrentNumberScheduled != ds.Status.NumberReady {
		return false, nil
	}

	return true, nil
}

// Build a stage pod based on `pod`.
func (t *Task) buildStagePod(pod *corev1.Pod) *corev1.Pod {
	labels := t.Owner.GetCorrelationLabels()
	labels[IncludedInStageBackupLabel] = t.UID()

	// Map of Restic volumes.
	resticVolumes := map[string]bool{}
	for _, name := range strings.Split(pod.Annotations[ResticPvBackupAnnotation], ",") {
		resticVolumes[name] = true
	}
	// Base pod.
	newPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.Namespace,
			Name:      pod.Name + "-" + "stage",
			Annotations: map[string]string{
				ResticPvBackupAnnotation: pod.Annotations[ResticPvBackupAnnotation],
			},
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers:                   []corev1.Container{},
			Volumes:                      []corev1.Volume{},
			SecurityContext:              pod.Spec.SecurityContext,
			ServiceAccountName:           pod.Spec.ServiceAccountName,
			AutomountServiceAccountToken: pod.Spec.AutomountServiceAccountToken,
		},
	}
	// Set NodeName
	newPod.Spec.NodeName = pod.Spec.NodeName
	// Add volumes.
	for _, volume := range pod.Spec.Volumes {
		if _, found := resticVolumes[volume.Name]; found {
			newPod.Spec.Volumes = append(newPod.Spec.Volumes, volume)
		}
	}
	// Add containers.
	for i, container := range pod.Spec.Containers {
		stageContainer := corev1.Container{
			Name:         "sleep-" + strconv.Itoa(i),
			Image:        "registry.access.redhat.com/rhel7",
			Command:      []string{"sleep"},
			Args:         []string{"infinity"},
			VolumeMounts: []corev1.VolumeMount{},
		}
		for _, mount := range container.VolumeMounts {
			if _, found := resticVolumes[mount.Name]; found {
				stageContainer.VolumeMounts = append(stageContainer.VolumeMounts, mount)
			}
		}
		newPod.Spec.Containers = append(newPod.Spec.Containers, stageContainer)
	}

	return &newPod
}

// Ensure the stage pods have been created.
func (t *Task) ensureStagePodsCreated() (int, error) {
	count := 0
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return 0, err
	}

	labelSelector := map[string]string{
		ApplicationPodLabel: t.UID(),
	}
	podList := corev1.PodList{}
	options := k8sclient.MatchingLabels(labelSelector)
	err = client.List(context.TODO(), options, &podList)
	if err != nil {
		log.Trace(err)
		return 0, err
	}
	cLabel, _ := t.Owner.GetCorrelationLabel()
	for _, pod := range podList.Items {
		if _, found := pod.Annotations[ResticPvBackupAnnotation]; !found {
			continue
		}
		// Skip stage pods.
		if _, found := pod.Labels[cLabel]; found {
			continue
		}
		// Create
		count++
		newPod := t.buildStagePod(&pod)
		err = client.Create(context.TODO(), newPod)
		if err == nil {
			log.Info(
				"Stage pod created.",
				"ns",
				newPod.Namespace,
				"name",
				newPod.Name)
			delete(pod.Annotations, ResticPvBackupAnnotation)
			err = client.Update(context.TODO(), &pod)
			if err != nil {
				log.Trace(err)
				return 0, err
			}
			continue
		}
		if !errors.IsAlreadyExists(err) {
			log.Trace(err)
			return 0, err
		}
	}

	if count > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StagePodsCreated,
			Status:   True,
			Reason:   Created,
			Category: Advisory,
			Message:  "[] Stage pods created.",
			Items:    []string{strconv.Itoa(count)},
			Durable:  true,
		})
	}

	return count, nil
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
	clients, namespaceList, err := t.getBothClientsWithNamespaces()
	if err != nil {
		log.Trace(err)
		return err
	}
	cLabel, _ := t.Owner.GetCorrelationLabel()
	for i, client := range clients {
		podList := corev1.PodList{}
		for _, ns := range namespaceList[i] {
			options := k8sclient.InNamespace(ns)
			err := client.List(context.TODO(), options, &podList)
			if err != nil {
				return err
			}
			for _, pod := range podList.Items {
				// Owned by ANY migration.
				if pod.Labels == nil {
					continue
				}
				if _, found := pod.Labels[cLabel]; !found {
					continue
				}
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
	}

	t.Owner.Status.DeleteCondition(StagePodsCreated)

	return nil
}

// Find all velero pods on the specified cluster.
func (t *Task) findVeleroPods(cluster *migapi.MigCluster) ([]corev1.Pod, error) {
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	list := &corev1.PodList{}
	selector := labels.SelectorFromSet(
		map[string]string{
			"component": "velero",
		})
	err = client.List(
		context.TODO(),
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: selector,
		},
		list)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	return list.Items, nil
}

// Ensure the velero cloud-credentials secret content has been
// mounted into all velero pods on both source and destination clusters.
func (t *Task) veleroPodCredSecretPropagated(cluster *migapi.MigCluster) (bool, error) {
	list, err := t.findVeleroPods(cluster)
	if err != nil {
		log.Trace(err)
		return false, err
	}
	if len(list) == 0 {
		log.Info("No velero pods found.")
		return false, nil
	}
	restCfg, err := cluster.BuildRestConfig(t.Client)
	if err != nil {
		log.Trace(err)
		return false, err
	}
	for _, pod := range list {
		storage, err := t.PlanResources.MigPlan.GetStorage(t.Client)
		if err != nil {
			log.Trace(err)
			return false, err
		}

		bslProvider := storage.GetBackupStorageProvider()
		vslProvider := storage.GetVolumeSnapshotProvider()
		for _, provider := range []pvdr.Provider{bslProvider, vslProvider} {
			cmd := pods.PodCommand{
				Args:    []string{"cat", provider.GetCloudCredentialsPath()},
				RestCfg: restCfg,
				Pod:     &pod,
			}
			err = cmd.Run()
			if err != nil {
				exErr, cast := err.(exec.CodeExitError)
				if cast && exErr.Code == 126 {
					log.Info(
						"Pod command failed:",
						"solution",
						"https://access.redhat.com/solutions/3734981",
						"cmd",
						cmd.Args)
					return true, nil
				} else {
					log.Trace(err)
					return false, err
				}
			}
			client, err := cluster.GetClient(t.Client)
			if err != nil {
				log.Trace(err)
				return false, err
			}
			secret, err := t.PlanResources.MigPlan.GetCloudSecret(client, provider)
			if err != nil {
				log.Trace(err)
				return false, err
			}
			if body, found := secret.Data["cloud"]; found {
				a := string(body)
				b := cmd.Out.String()
				if a != b {
					return false, nil
				}
			} else {
				return false, nil
			}
		}
	}

	return true, nil
}
