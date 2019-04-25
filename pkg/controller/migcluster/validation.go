package migcluster

import (
	"context"
	"fmt"
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
	TestConnectFailed  = "TestConnectFailed"
)

// Reasons
const (
	NotSet        = "NotSet"
	NotFound      = "NotFound"
	ConnectFailed = "ConnectFailed"
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
	TestConnectFailedMessage  = "Test connect failed: %s"
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigCluster) validate(cluster *migapi.MigCluster) (int, error) {
	totalSet := 0
	var err error
	nSet := 0

	// registry cluster
	nSet, err = r.validateRegistryCluster(cluster)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// SA secret
	nSet, err = r.validateSaSecret(cluster)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Test Connection
	nSet, err = r.testConnection(cluster, totalSet)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Ready
	cluster.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes.
	err = r.Update(context.TODO(), cluster)
	if err != nil {
		return 0, err
	}

	return totalSet, err
}

func (r ReconcileMigCluster) validateRegistryCluster(cluster *migapi.MigCluster) (int, error) {
	ref := cluster.Spec.ClusterRef

	// Not needed.
	if cluster.Spec.IsHostCluster {
		cluster.Status.DeleteCondition(InvalidClusterRef)
		return 0, nil
	}

	// NotSet
	if !migref.RefSet(ref) {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidClusterRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidClusterRefMessage,
		})
		return 1, nil
	}

	storage, err := r.getCluster(ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if storage == nil {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidClusterRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidClusterRefMessage,
		})
		return 1, nil
	} else {
		cluster.Status.DeleteCondition(InvalidClusterRef)
	}

	return 0, nil
}

func (r ReconcileMigCluster) getCluster(ref *kapi.ObjectReference) (*crapi.Cluster, error) {
	if ref == nil {
		return nil, nil
	}
	cluster := crapi.Cluster{}
	err := r.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		&cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &cluster, err
}

func (r ReconcileMigCluster) validateSaSecret(cluster *migapi.MigCluster) (int, error) {
	ref := cluster.Spec.ServiceAccountSecretRef

	// Not needed.
	if cluster.Spec.IsHostCluster {
		cluster.Status.DeleteCondition(InvalidSaSecretRef)
		cluster.Status.DeleteCondition(InvalidSaToken)
		return 0, nil
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
		return 1, nil
	}

	secret, err := migapi.GetSecret(r, ref)
	if err != nil {
		return 0, err
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
		return 1, nil
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
		return 1, nil
	}
	if len(token) == 0 {
		cluster.Status.SetCondition(migapi.Condition{
			Type:    InvalidSaToken,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidSaTokenMessage,
		})
		return 1, nil
	} else {
		cluster.Status.DeleteCondition(InvalidSaToken)
	}

	return 0, nil
}

// Test the connection.
func (r ReconcileMigCluster) testConnection(cluster *migapi.MigCluster, totalSet int) (int, error) {
	if cluster.Spec.IsHostCluster {
		cluster.Status.DeleteCondition(TestConnectFailed)
		return 0, nil
	}
	if totalSet > 0 {
		return 0, nil
	}
	_, err := cluster.GetClient(r)
	if err != nil {
		message := fmt.Sprintf(TestConnectFailedMessage, err)
		cluster.Status.SetCondition(migapi.Condition{
			Type:    TestConnectFailed,
			Status:  True,
			Reason:  ConnectFailed,
			Message: message,
		})
		return 1, nil
	} else {
		cluster.Status.DeleteCondition(TestConnectFailed)
	}

	return 0, nil
}
