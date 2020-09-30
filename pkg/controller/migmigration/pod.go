package migmigration

import (
	"context"
	"fmt"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	pvdr "github.com/konveyor/mig-controller/pkg/cloudprovider"
	"github.com/konveyor/mig-controller/pkg/pods"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/exec"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Determine if restic should restart
func (t *Task) shouldResticRestart() (bool, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}

	// Default to running ResticRestart on 3.7, 3.9.
	// ResticRestart not needed for default feature gates settings
	// on 3.10, 3.11, 4.x+. Could be needed if gates re-configured.
	runRestart := false
	if client.MajorVersion() == 1 && client.MinorVersion() == 7 {
		runRestart = true
	}
	if client.MajorVersion() == 1 && client.MinorVersion() == 9 {
		runRestart = true
	}
	// User can override default by setting MigCluster.Spec.RestartRestic.
	if t.PlanResources.SrcMigCluster.Spec.RestartRestic != nil {
		runRestart = *t.PlanResources.SrcMigCluster.Spec.RestartRestic
	}
	return runRestart, nil
}

// Delete the running restic pods.
// Restarted to get around mount propagation requirements.
func (t *Task) restartResticPods() error {
	// Verify restic restart is needed before proceeding
	runRestart, err := t.shouldResticRestart()
	if err != nil {
		return liberr.Wrap(err)
	}
	if !runRestart {
		return nil
	}

	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
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
		return liberr.Wrap(err)
	}

	for _, pod := range list.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		err = client.Delete(
			context.TODO(),
			&pod)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Determine if restic pod is running.
func (t *Task) haveResticPodsStarted() (bool, error) {
	progress := []string{}
	// Verify restic restart is needed before proceeding
	runRestart, err := t.shouldResticRestart()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if !runRestart {
		return true, nil
	}

	client, err := t.getSourceClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}

	list := corev1.PodList{}
	ds := appsv1.DaemonSet{}
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
		return false, liberr.Wrap(err)
	}

	err = client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "restic",
			Namespace: migapi.VeleroNamespace,
		},
		&ds)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	restarted := true
	for _, pod := range list.Items {
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
			restarted = false
			progress = append(
				progress,
				fmt.Sprintf(
					"Restic Pod %s/%s: Not running yet",
					pod.Namespace,
					pod.Name))
		}
	}
	if ds.Status.CurrentNumberScheduled != ds.Status.NumberReady {
		restarted = false
		progress = append(
			progress,
			fmt.Sprintf(
				"Restic DaemonSet %s/%s: Desired number of pods not ready yet",
				ds.Namespace,
				ds.Name))
	}

	return restarted, nil
}

// Find all velero pods on the specified cluster.
func (t *Task) findVeleroPods(cluster *migapi.MigCluster) ([]corev1.Pod, error) {
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
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
		return nil, liberr.Wrap(err)
	}

	return list.Items, nil
}

// Ensure the velero cloud-credentials secret content has been
// mounted into all velero pods on both source and destination clusters.
func (t *Task) veleroPodCredSecretPropagated(cluster *migapi.MigCluster) (bool, error) {
	list, err := t.findVeleroPods(cluster)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if len(list) == 0 {
		log.Info("No velero pods found.")
		return false, nil
	}
	restCfg, err := cluster.BuildRestConfig(t.Client)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	for _, pod := range list {
		storage, err := t.PlanResources.MigPlan.GetStorage(t.Client)
		if err != nil {
			return false, liberr.Wrap(err)
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
					return false, liberr.Wrap(err)
				}
			}
			client, err := cluster.GetClient(t.Client)
			if err != nil {
				return false, liberr.Wrap(err)
			}
			secret, err := t.PlanResources.MigPlan.GetCloudSecret(client, provider)
			if err != nil {
				return false, liberr.Wrap(err)
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
