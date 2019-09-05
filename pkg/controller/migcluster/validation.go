package migcluster

import (
	"fmt"
	"time"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
)

// Types
const (
	InvalidURL         = "InvalidURL"
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
	InvalidURLMessage         = "The `url` is required when `isHostCluster` is false."
	InvalidSaSecretRefMessage = "The `serviceAccountSecretRef` must reference a `secret`."
	InvalidSaTokenMessage     = "The `saToken` not found in `serviceAccountSecretRef` secret."
	TestConnectFailedMessage  = "Test connect failed: %s"
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigCluster) validate(cluster *migapi.MigCluster) error {
	// General settings
	err := r.validateURL(cluster)
	if err != nil {
		log.Trace(err)
		return err
	}

	// SA secret
	err = r.validateSaSecret(cluster)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Test Connection
	err = r.testConnection(cluster)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

func (r ReconcileMigCluster) validateURL(cluster *migapi.MigCluster) error {
	// Not needed.
	if cluster.Spec.IsHostCluster {
		return nil
	}
	if cluster.Spec.URL == "" {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidURL,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidURLMessage,
		})
	}

	return nil
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
		log.Trace(err)
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
	token, found := secret.Data[migapi.SaToken]
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

	// Timeout of 5s instead of the default 30s to lessen lockup
	timeout := time.Duration(time.Second * 5)
	err := cluster.TestConnection(r.Client, timeout)
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
