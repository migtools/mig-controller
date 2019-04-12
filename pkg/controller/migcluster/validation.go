package migcluster

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crapi "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
)

// Types
const (
	InvalidClusterRef  = "InvalidClusterRef"
	InvalidSaSecretRef = "InvalidSaSecretRef"
	InvalidSaToken     = "InvalidSaToken"
)

// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage              = "The cluster is ready."
	InvalidClusterRefMessage  = "The `clusterRef` must reference a `cluster`."
	InvalidSaSecretRefMessage = "The `serviceAccountSecretRef` must reference a `secret`."
	InvalidSaTokenMessage     = "The `saToken` not found in `serviceAccountSecretRef` secret."
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigCluster) validate(cluster *migapi.MigCluster) (error, int) {
	totalSet := 0
	var err error
	nSet := 0

	// registry cluster
	err, nSet = r.validateRegistryCluster(cluster)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// SA secret
	err, nSet = r.validateSaSecret(cluster)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Ready
	cluster.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes.
	err = r.Update(context.TODO(), cluster)
	if err != nil {
		return err, 0
	}

	return err, totalSet
}

func (r ReconcileMigCluster) validateRegistryCluster(cluster *migapi.MigCluster) (error, int) {
	ref := cluster.Spec.ClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidClusterRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidClusterRefMessage,
		})
		return nil, 1
	}

	err, storage := r.getCluster(ref)
	if err != nil {
		return err, 0
	}

	// NotFound
	if storage == nil {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidClusterRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidClusterRefMessage,
		})
		return nil, 1
	} else {
		cluster.Status.DeleteCondition(InvalidClusterRef)
	}

	return nil, 0
}

func (r ReconcileMigCluster) getCluster(ref *kapi.ObjectReference) (error, *crapi.Cluster) {
	key := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}

	cluster := crapi.Cluster{}
	err := r.Get(context.TODO(), key, &cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return err, nil
		}
	}

	return nil, &cluster
}

func (r ReconcileMigCluster) validateSaSecret(cluster *migapi.MigCluster) (error, int) {
	ref := cluster.Spec.ServiceAccountSecretRef

	if cluster.Spec.IsHostCluster {
		cluster.Status.DeleteCondition(InvalidSaSecretRef)
		cluster.Status.DeleteCondition(InvalidSaToken)
		return nil, 0
	}

	// NotSet
	if !migref.RefSet(ref) {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidSaSecretRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidSaSecretRefMessage,
		})
		cluster.Status.DeleteCondition(InvalidSaToken)
		return nil, 1
	}

	err, secret := r.getSecret(ref)
	if err != nil {
		return err, 0
	}

	// NotFound
	if secret == nil {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidSaSecretRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidSaSecretRefMessage,
		})
		cluster.Status.DeleteCondition(InvalidSaToken)
		return nil, 1
	} else {
		cluster.Status.DeleteCondition(InvalidSaSecretRef)
	}

	// saToken
	token, found := secret.Data["saToken"]
	if !found {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidSaToken,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidSaTokenMessage,
		})
		return nil, 1
	}
	if len(token) == 0 {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidSaToken,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidSaTokenMessage,
		})
		return nil, 1
	} else {
		cluster.Status.DeleteCondition(InvalidSaToken)
	}

	return nil, 0
}

func (r ReconcileMigCluster) getSecret(ref *kapi.ObjectReference) (error, *kapi.Secret) {
	key := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}

	secret := kapi.Secret{}
	err := r.Get(context.TODO(), key, &secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return err, nil
		}
	}

	return nil, &secret
}
