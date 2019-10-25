package migmigration

import (
	"context"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/fusor/mig-controller/pkg/pods"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	list := corev1.PodList{}
	ds := v1beta1.DaemonSet{}
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

	err = client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "restic",
			Namespace: migapi.VeleroNamespace,
		},
		&ds)
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

// Ensure the stage pods have been deleted.
func (t *Task) ensureStagePodsDeleted() error {
	client, err := t.getDestinationClient()
	namespaceList := t.destinationNamespaces()
	if err != nil {
		log.Trace(err)
		return err
	}
	podList := corev1.PodList{}
	for _, ns := range namespaceList {
		selector := labels.SelectorFromSet(map[string]string{
			StagePodLabel: t.UID(),
		})
		err := client.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&podList)
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

// Find all velero pods on the specified cluster.
func (t *Task) findVeleroPods(cluster *migapi.MigCluster) ([]corev1.Pod, error) {
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	list := &corev1.PodList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(
			map[string]string{"component": "velero"}),
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
		cmd := pods.PodCommand{
			Args:    []string{"cat", "/credentials/cloud"},
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
		secret, err := t.PlanResources.MigPlan.GetCloudSecret(client)
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

	return true, nil
}

// Ensure the stage pods have been deleted.
func (t *Task) ensureStagePodsTerminated() (bool, error) {
	client, err := t.getDestinationClient()
	namespaceList := t.destinationNamespaces()
	if err != nil {
		log.Trace(err)
		return false, err
	}
	podList := corev1.PodList{}
	for _, ns := range namespaceList {
		selector := labels.SelectorFromSet(map[string]string{
			StagePodLabel: t.UID(),
		})
		err := client.List(
			context.TODO(),
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			},
			&podList)
		if err != nil {
			return false, err
		}
		for _, pod := range podList.Items {
			log.Info("Waiting for stage pod ", pod.Name, " to terminate")
			return false, nil
		}
	}

	t.Owner.Status.DeleteCondition(EnsureStagePodsDeleted)

	return true, nil
}
