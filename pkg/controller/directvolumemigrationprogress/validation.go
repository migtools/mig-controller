package directvolumemigrationprogress

import (
	"fmt"
	"path"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/api/errors"
)

// Condition reasons
const (
	NotFound    = "NotFound"
	NotSet      = "NotSet"
	NotDistinct = "NotDistinct"
	NotReady    = "NotReady"
)

// Condition types
const (
	InvalidClusterRef  = "InvalidClusterRef"
	ClusterNotReady    = "ClusterNotReady"
	InvalidPodRef      = "InvalidPodRef"
	InvalidPod         = "InvalidPod"
	PodNotReady        = "PodNotReady"
	InvalidSpec        = "InvalidSpec"
	InvalidPodSelector = "InvalidPodSelector"
)

func (r *ReconcileDirectVolumeMigrationProgress) validate(pvProgress *migapi.DirectVolumeMigrationProgress) (
	cluster *migapi.MigCluster, client compat.Client, err error) {
	cluster, err = r.validateCluster(pvProgress)
	if err != nil {
		log.V(4).Info("Validation check for referenced MigCluster failed", err)
		err = liberr.Wrap(err)
		return
	}
	if cluster == nil {
		return
	}
	client, err = cluster.GetClient(r)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.validateSpec(client, pvProgress)
	if err != nil {
		log.V(4).Info("Validation check for spec failed", "error", err)
		err = liberr.Wrap(err)
		return
	}
	return
}

func (r *ReconcileDirectVolumeMigrationProgress) validateSpec(srcClient compat.Client, pvProgress *migapi.DirectVolumeMigrationProgress) (err error) {
	podRef := pvProgress.Spec.PodRef
	podSelector := pvProgress.Spec.PodSelector
	podNamespace := pvProgress.Spec.PodNamespace
	if podRef != nil && podRef.Namespace != "" && podSelector == nil {
		_, err = getPod(srcClient, podRef)
		switch {
		case errors.IsNotFound(err):
			// handle
			pvProgress.Status.SetCondition(migapi.Condition{
				Type:     InvalidPod,
				Status:   migapi.True,
				Reason:   NotFound,
				Category: migapi.Critical,
				Message: fmt.Sprintf("The spec.podRef %s must reference a valid `Pod` ",
					path.Join(podRef.Namespace, podRef.Name)),
			})
		default:
			err = liberr.Wrap(err)
		}
	} else if podSelector != nil && podNamespace != "" {
		// return if the pod identity label is not present in the pod selector, cannot find Rsync pods
		if _, exists := podSelector[migapi.RsyncPodIdentityLabel]; !exists {
			pvProgress.Status.SetCondition(migapi.Condition{
				Type:     InvalidPodSelector,
				Category: migapi.Critical,
				Message:  "spec.PodSelector missing required pod identity label, cannot disover Direct Volume Migration pods.",
				Status:   migapi.True,
				Reason:   NotSet,
			})
		}
	} else {
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     InvalidSpec,
			Status:   migapi.True,
			Reason:   NotSet,
			Category: migapi.Critical,
			Message:  "spec.PodRef and/or spec.PodSelector missing, cannot discover Direct Volume Migration pods.",
		})
	}
	return
}

func (r *ReconcileDirectVolumeMigrationProgress) validateCluster(pvProgress *migapi.DirectVolumeMigrationProgress) (cluster *migapi.MigCluster, err error) {
	clusterRef := pvProgress.Spec.ClusterRef
	// NotSet
	if !migref.RefSet(clusterRef) {
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     InvalidClusterRef,
			Status:   migapi.True,
			Reason:   NotSet,
			Category: migapi.Critical,
			Message:  "The spec.clusterRef must reference name and namespace of a valid `MigCluster",
		})
		return
	}

	cluster, err = migapi.GetCluster(r, clusterRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// NotFound
	if cluster == nil {
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     InvalidClusterRef,
			Status:   migapi.True,
			Reason:   NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The spec.clusterRef must reference a valid `MigCluster` %s",
				path.Join(clusterRef.Namespace, clusterRef.Name)),
		})
		return
	}

	// Not ready
	if !cluster.Status.IsReady() {
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     ClusterNotReady,
			Status:   migapi.True,
			Reason:   NotReady,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The `MigCluster` spec.ClusterRef %s is not ready",
				path.Join(clusterRef.Namespace, clusterRef.Name)),
		})
	}
	return
}
