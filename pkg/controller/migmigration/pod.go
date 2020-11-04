package migmigration

import (
	"context"

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

// Determine if velero should restart based on:
// 1. if we deleted pending Velero CRs
func (t *Task) shouldVeleroRestart() bool {
	if t.Owner.Status.HasCondition(PendingVeleroCRsDeleted) {
		return true
	}
	return false
}

// Determine if restic should restart based on:
// 1. if we deleted pending Velero CRs
// 2. if k8s version requires mount propagation workaround.
func (t *Task) shouldResticRestart() (bool, error) {
	// 1. Check if we deleted pending Velero CRs, need to do restic restart.
	if t.Owner.Status.HasCondition(PendingVeleroCRsDeleted) {
		return true, nil
	}

	// 2. Check if k8s version requires mount propagation workaround
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

	clients, err := t.getBothClients()
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, client := range clients {
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
	}

	return nil
}

// Determine if restic pod is running.
func (t *Task) haveResticPodsStarted() (bool, error) {
	// Verify restic restart is needed before proceeding
	runRestart, err := t.shouldResticRestart()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if !runRestart {
		return true, nil
	}

	clients, err := t.getBothClients()
	if err != nil {
		return false, liberr.Wrap(err)
	}

	for _, client := range clients {
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
	}

	return true, nil
}

// Delete the running Velero pods.
// Needed to stop any pending tasks after Velero CR deletion
func (t *Task) restartVeleroPods() error {
	runRestart := t.shouldVeleroRestart()
	if !runRestart {
		return nil
	}

	// Delete Velero Pods on both clusters
	clusters := t.getBothClusters()
	for _, cluster := range clusters {
		veleroPods, err := t.findVeleroPods(cluster)
		if err != nil {
			return liberr.Wrap(err)
		}
		clusterClient, err := cluster.GetClient(t.Client)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, pod := range veleroPods {
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}
			err = clusterClient.Delete(
				context.TODO(),
				&pod)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Determine if Velero Pod is running.
func (t *Task) haveVeleroPodsStarted() (bool, error) {
	// Verify Velero restart is needed before continuing check
	runRestart := t.shouldVeleroRestart()
	if !runRestart {
		return true, nil
	}

	clients, err := t.getBothClients()
	if err != nil {
		return false, liberr.Wrap(err)
	}

	for _, client := range clients {
		list := corev1.PodList{}
		deployment := appsv1.Deployment{}
		selector := labels.SelectorFromSet(map[string]string{
			"component": "velero",
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
				Name:      "velero",
				Namespace: migapi.VeleroNamespace,
			},
			&deployment)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		for _, pod := range list.Items {
			if pod.DeletionTimestamp != nil {
				return false, nil
			}
			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}
		}
		if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
			return false, nil
		}
	}

	return true, nil
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
