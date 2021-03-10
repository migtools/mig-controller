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
		t.Log.Info("Detected OpenShift 3.7. Restic will restart unless override is set.")
	}
	if client.MajorVersion() == 1 && client.MinorVersion() == 9 {
		runRestart = true
		t.Log.Info("Detected OpenShift 3.9. Restic will restart unless override is set.")
	}
	// User can override default by setting MigCluster.Spec.RestartRestic.
	if t.PlanResources.SrcMigCluster.Spec.RestartRestic != nil {
		runRestart = *t.PlanResources.SrcMigCluster.Spec.RestartRestic
		t.Log.Info(fmt.Sprintf("SrcCluster.Spec.RestartRestic override [%v] found.",
			runRestart))
	}
	return runRestart, nil
}

// Delete the running restic pods.
// Restarted to get around mount propagation requirements.
func (t *Task) restartResticPods() error {
	// Verify restic restart is needed before proceeding
	t.Log.Info("Checking if Restic Restart is required.")
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
	t.Log.Info(fmt.Sprintf("Getting Restic Pods on source cluster in namespace [%v]",
		migapi.VeleroNamespace))
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
			t.Log.Info(fmt.Sprintf("Found Restic Pod [%v/%v] in non-running state, skipping restart.",
				pod.Namespace, pod.Name))
			continue
		}
		t.Log.Info(fmt.Sprintf("Deleting Restic Pod [%v/%v] in source cluster to trigger restart.",
			pod.Namespace, pod.Name))
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

	t.Log.Info("Getting Restic DaemonSet on source cluster")
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

	t.Log.Info("Getting list of Restic Pods on source cluster")
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
			t.Log.Info(fmt.Sprintf("Deletion timestamp found on Restic Pod [%v/%v], "+
				"Pod is in the process of deleting. Requeuing and waiting for restart.",
				pod.Namespace, pod.Name))
			return false, nil
		}
		if pod.Status.Phase != corev1.PodRunning {
			t.Log.Info(fmt.Sprintf("Found Restic Pod [%v/%v] in non-running state."+
				" Requeuing and waiting for restart.", pod.Namespace, pod.Name))
			return false, nil
		}
	}
	if ds.Status.CurrentNumberScheduled != ds.Status.NumberReady {
		t.Log.Info(fmt.Sprintf("Restic DaemonSet [%v/%v] .Status.CurrentNumberScheduled "+
			" differs from .Status.NumberReady. Requeuing and waiting for these to match.",
			ds.Namespace, ds.Name))
		return false, nil
	}

	return true, nil
}

// Delete the running Velero pods.
// Needed to stop any pending tasks after Velero CR deletion
func (t *Task) restartVeleroPods() error {
	// Restart source cluster Velero pod if needed
	if t.Owner.Status.HasCondition(StaleSrcVeleroCRsDeleted) {
		err := t.deleteVeleroPodsForCluster(t.PlanResources.SrcMigCluster)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	// Restart target cluster Velero pod if needed
	if t.Owner.Status.HasCondition(StaleDestVeleroCRsDeleted) {
		err := t.deleteVeleroPodsForCluster(t.PlanResources.DestMigCluster)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

func (t *Task) deleteVeleroPodsForCluster(cluster *migapi.MigCluster) error {
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
	return nil
}

// Determine if Velero Pod is running.
func (t *Task) haveVeleroPodsStarted() (bool, error) {
	// Verify Velero restart was needed before performing check
	if !t.Owner.Status.HasAnyCondition(StaleSrcVeleroCRsDeleted, StaleDestVeleroCRsDeleted) {
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
				t.Log.Info(fmt.Sprintf("Found Velero Pod [%v/%v] with deletion timestamp."+
					" Requeuing and waiting for Pod to finish deleting and restart.",
					pod.Namespace, pod.Name))
				return false, nil
			}
			if pod.Status.Phase != corev1.PodRunning {
				t.Log.Info(fmt.Sprintf("Found Velero Pod [%v/%v] with Status.Phase != Running."+
					" Requeuing and waiting for Pod to enter running state.",
					pod.Namespace, pod.Name))
				return false, nil
			}
		}
		if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
			t.Log.Info(fmt.Sprintf("Found Velero Deployment [%v/%v] with "+
				"Spec.Replicas != Status.ReadyReplicas. Requeuing and waiting for these fields to match.",
				deployment.Namespace, deployment.Name))
			return false, nil
		}
	}

	// Remove the condition notifying the user that Velero will be restarted
	t.Owner.Status.DeleteCondition(StaleSrcVeleroCRsDeleted)
	t.Owner.Status.DeleteCondition(StaleDestVeleroCRsDeleted)
	return true, nil
}

// Find all velero pods on the specified cluster.
func (t *Task) findVeleroPods(cluster *migapi.MigCluster) ([]corev1.Pod, error) {
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return pods.FindVeleroPods(client)
}

// Ensure the velero cloud-credentials secret content has been
// mounted into all velero pods on both source and destination clusters.
func (t *Task) veleroPodCredSecretPropagated(cluster *migapi.MigCluster) (bool, error) {
	list, err := t.findVeleroPods(cluster)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if len(list) == 0 {
		t.Log.Info(fmt.Sprintf("No velero pods found on MigCluster [%v/%v] "+
			"while checking for Velero cloud secret propagation.",
			cluster.Namespace, cluster.Name))
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
			t.Log.Info(fmt.Sprintf("Execing into Velero Pod [%v/%v]"+
				"on MigCluster [%v/%v] with command [%v] to check for Cloud Credentials",
				pod.Namespace, pod.Name, cluster.Namespace, cluster.Name,
				"cat "+provider.GetCloudCredentialsPath()))
			err = cmd.Run()
			if err != nil {
				exErr, cast := err.(exec.CodeExitError)
				if cast && exErr.Code == 126 {
					t.Log.Info(
						"Exec into Velero Pod command failed:",
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
	t.Log.Info(fmt.Sprintf("Found propagated cloud secret in Velero Pod "+
		"on MigCluster [%v/%v]", cluster.Name, cluster.Namespace))
	return true, nil
}
