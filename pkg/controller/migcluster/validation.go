package migcluster

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	auth "k8s.io/api/authorization/v1"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
)

// Types
const (
	InvalidURL           = "InvalidURL"
	InvalidSaSecretRef   = "InvalidSaSecretRef"
	InvalidSaToken       = "InvalidSaToken"
	TestConnectFailed    = "TestConnectFailed"
	SaTokenNotPrivileged = "SaTokenNotPrivileged"
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
	Malformed     = "Malformed"
	InvalidScheme = "InvalidScheme"
	Unauthorized  = "Unauthorized"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage                = "The cluster is ready."
	MissingURLMessage           = "The `url` is required when `isHostCluster` is false."
	CaBundleMessage             = "The `caBundle` may be required when `insecure` is false."
	InvalidSaSecretRefMessage   = "The `serviceAccountSecretRef` must reference a `secret`."
	InvalidSaTokenMessage       = "The `saToken` not found in `serviceAccountSecretRef` secret."
	TestConnectFailedMessage    = "Test connect failed. %s: %s"
	MalformedURLMessage         = "The `url` is malformed."
	InvalidURLSchemeMessage     = "The `url` scheme must be 'http' or 'https'."
	SaTokenNotPrivilegedMessage = "The `saToken` has insufficient privileges."
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigCluster) validate(cluster *migapi.MigCluster) error {
	// General settings
	err := r.validateURL(cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// SA secret
	err = r.validateSaSecret(cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Test Connection
	err = r.testConnection(cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Token privileges
	err = r.validateSaTokenPrivileges(cluster)
	if err != nil {
		return liberr.Wrap(err)
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
			Message:  MissingURLMessage,
		})
		return nil
	}
	u, err := url.Parse(cluster.Spec.URL)
	if err != nil {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidURL,
			Status:   True,
			Reason:   Malformed,
			Category: Critical,
			Message:  MalformedURLMessage,
		})
		return nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidURL,
			Status:   True,
			Reason:   InvalidScheme,
			Category: Critical,
			Message:  InvalidURLSchemeMessage,
		})
		return nil
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
		return liberr.Wrap(err)
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
		helpText := ""
		if strings.Contains(err.Error(), "x509") {
			helpText = CaBundleMessage
		}
		message := fmt.Sprintf(TestConnectFailedMessage, helpText, err)
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

func (r *ReconcileMigCluster) validateSaTokenPrivileges(cluster *migapi.MigCluster) error {
	if cluster.Spec.IsHostCluster {
		return nil
	}
	if cluster.Status.HasCriticalCondition() {
		return nil
	}

	// check for access to all verbs on all resources in all namespaces
	// in the migration.openshift.io and velero.io groups in order to
	// determine if the service account has sufficient permissions
	migrationSar := auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:    "migration.openshift.io",
				Resource: "*",
				Verb:     "*",
				Version:  "*",
			},
		},
	}

	veleroSar := auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:    "velero.io",
				Resource: "*",
				Verb:     "*",
				Version:  "*",
			},
		},
	}

	client, err := cluster.GetClient(r.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = client.Create(context.TODO(), &migrationSar)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = client.Create(context.TODO(), &veleroSar)
	if err != nil {
		return liberr.Wrap(err)
	}
	if !migrationSar.Status.Allowed || !veleroSar.Status.Allowed {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     SaTokenNotPrivileged,
			Status:   True,
			Reason:   Unauthorized,
			Category: Critical,
			Message:  SaTokenNotPrivilegedMessage,
		})
	}
	return nil
}
