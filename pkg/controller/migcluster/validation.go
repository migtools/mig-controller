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

// Categories
const (
	Critical = migapi.Critical
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
func (r ReconcileMigCluster) validate(cluster *migapi.MigCluster) error {
	cluster.Status.BeginStagingConditions()

	// registry cluster
	err := r.validateRegistryCluster(cluster)
	if err != nil {
		return err
	}

	// SA secret
	err = r.validateSaSecret(cluster)
	if err != nil {
		return err
	}

	// Test Connection
	err = r.testConnection(cluster)
	if err != nil {
		return err
	}

	// Ready
	cluster.Status.SetReady(
		!cluster.Status.HasBlockerCondition(),
		ReadyMessage)

	// Apply changes.
	cluster.Status.EndStagingConditions()
	err = r.Update(context.TODO(), cluster)
	if err != nil {
		return err
	}

	return nil
}

func (r ReconcileMigCluster) validateRegistryCluster(cluster *migapi.MigCluster) error {
	ref := cluster.Spec.ClusterRef

	// Not needed.
	if cluster.Spec.IsHostCluster {
		return nil
	}

	// NotSet
	if !migref.RefSet(ref) {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidClusterRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidClusterRefMessage,
		})
		return nil
	}

	storage, err := r.getCluster(ref)
	if err != nil {
		return err
	}

	// NotFound
	if storage == nil {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidClusterRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  InvalidClusterRefMessage,
		})
		return nil
	}

	return nil
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

func (r ReconcileMigCluster) validateSaSecret(cluster *migapi.MigCluster) error {
	ref := cluster.Spec.ServiceAccountSecretRef

	// Not needed.
	if cluster.Spec.IsHostCluster {
		return nil
	}

	// NotSet
	if !migref.RefSet(ref) {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidSaSecretRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidSaSecretRefMessage,
		})
		return nil
	}

	secret, err := migapi.GetSecret(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if secret == nil {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidSaSecretRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  InvalidSaSecretRefMessage,
		})
		return nil
	}

	// saToken
	token, found := secret.Data["saToken"]
	if !found {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidSaToken,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  InvalidSaTokenMessage,
		})
		return nil
	}
	if len(token) == 0 {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidSaToken,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidSaTokenMessage,
		})
		return nil
	}

	return nil
}

// Test the connection.
func (r ReconcileMigCluster) testConnection(cluster *migapi.MigCluster) error {
	if cluster.Spec.IsHostCluster {
		return nil
	}
	if cluster.Status.HasCriticalCondition() {
		return nil
	}
	_, err := cluster.GetClient(r)
	if err != nil {
		message := fmt.Sprintf(TestConnectFailedMessage, err)
		cluster.Status.SetCondition(migapi.Condition{
			Type:     TestConnectFailed,
			Status:   True,
			Reason:   ConnectFailed,
			Category: Critical,
			Message:  message,
		})
		return nil
	}

	return nil
}
