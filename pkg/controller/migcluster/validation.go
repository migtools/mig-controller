package migcluster

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	auth "k8s.io/api/authorization/v1"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Types
const (
	InvalidURL                     = "InvalidURL"
	InvalidSaSecretRef             = "InvalidSaSecretRef"
	InvalidSaToken                 = "InvalidSaToken"
	InvalidRegistryRoute           = "InvalidRegistryRoute"
	TestConnectFailed              = "TestConnectFailed"
	SaTokenNotPrivileged           = "SaTokenNotPrivileged"
	OperatorVersionMismatch        = "OperatorVersionMismatch"
	ClusterOperatorVersionNotFound = "ClusterOperatorVersionNotFound"
)

// Categories
const (
	Critical = migapi.Critical
	Warn     = migapi.Warn
)

// Reasons
const (
	NotSet             = "NotSet"
	NotFound           = "NotFound"
	ConnectFailed      = "ConnectFailed"
	Malformed          = "Malformed"
	RouteTestFailed    = "RouteTestFailed"
	InvalidScheme      = "InvalidScheme"
	Unauthorized       = "Unauthorized"
	VersionCheckFailed = "VersionCheckFailed"
	VersionNotFound    = "VersionNotFound"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigCluster) validate(ctx context.Context, cluster *migapi.MigCluster) error {
	if opentracing.SpanFromContext(ctx) != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validate")
		defer span.Finish()
	}

	// General settings
	err := r.validateURL(ctx, cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// SA secret
	err = r.validateSaSecret(ctx, cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Test Connection
	err = r.testConnection(ctx, cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Token privileges
	err = r.validateSaTokenPrivileges(ctx, cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Exposed registry route
	err = r.validateRegistryRoute(ctx, cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	// cluster version
	err = r.validateOperatorVersionMatchesHost(ctx, cluster)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

func (r ReconcileMigCluster) validateURL(ctx context.Context, cluster *migapi.MigCluster) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateURL")
		defer span.Finish()
	}

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
			Message:  "The `spec.URL` is required for non-host cluster.",
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
			Message:  "The `spec.URL` is malformed.",
		})
		return nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidURL,
			Status:   True,
			Reason:   InvalidScheme,
			Category: Critical,
			Message:  "The `url` scheme is invalid, must be: (http|https).",
		})
		return nil
	}
	return nil
}

func (r ReconcileMigCluster) validateSaSecret(ctx context.Context, cluster *migapi.MigCluster) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateSaSecret")
		defer span.Finish()
	}

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
			Message:  "The `serviceAccountSecretRef` must reference a `secret`.",
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
			Message: fmt.Sprintf("The `serviceAccountSecretRef` must reference a valid `secret`,"+
				" subject: %s.", path.Join(cluster.Spec.ServiceAccountSecretRef.Namespace,
				cluster.Spec.ServiceAccountSecretRef.Name)),
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
			Message: fmt.Sprintf("The `saToken` not found in `serviceAccountSecretRef` secret,"+
				" subject: %s.", path.Join(cluster.Spec.ServiceAccountSecretRef.Namespace,
				cluster.Spec.ServiceAccountSecretRef.Name)),
		})
		return nil
	}
	if len(token) == 0 {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     InvalidSaToken,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message: fmt.Sprintf("The `saToken` found in `serviceAccountSecretRef` secret is empty,"+
				" subject: %s.", path.Join(cluster.Spec.ServiceAccountSecretRef.Namespace,
				cluster.Spec.ServiceAccountSecretRef.Name)),
		})
		return nil
	}

	return nil
}

// Test the connection.
func (r ReconcileMigCluster) testConnection(ctx context.Context, cluster *migapi.MigCluster) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "testConnection")
		defer span.Finish()
	}

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
		if strings.Contains(err.Error(), "x509") &&
			len(cluster.Spec.CABundle) == 0 && !cluster.Spec.Insecure {
			helpText = "The `caBundle` is required for self-signed API server certificates."
		}
		cluster.Status.SetCondition(migapi.Condition{
			Type:     TestConnectFailed,
			Status:   True,
			Reason:   ConnectFailed,
			Category: Critical,
			Message:  fmt.Sprintf("Test connect failed. %s", helpText),
			Items:    []string{err.Error()},
		})
		return nil
	}

	return nil
}

// Validate the Exposed registry route
func (r ReconcileMigCluster) validateRegistryRoute(ctx context.Context, cluster *migapi.MigCluster) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateRegistryRoute")
		defer span.Finish()
	}

	if cluster.Status.HasCriticalCondition() {
		return nil
	}

	if cluster.Spec.ExposedRegistryPath != "" {
		statusCode, regErr, err := checkRegistryConnection(cluster, r.Client)
		if err != nil {
			return liberr.Wrap(err)
		}
		if regErr != nil {
			cluster.Status.SetCondition(migapi.Condition{
				Type:     InvalidRegistryRoute,
				Status:   True,
				Reason:   RouteTestFailed,
				Category: Critical,
				Message:  fmt.Sprintf("Exposed registry route is invalid, Error : %#v", regErr.Error()),
				Items:    []string{err.Error()},
			})
			return nil
		}

		if statusCode != 200 {
			cluster.Status.SetCondition(migapi.Condition{
				Type:     InvalidRegistryRoute,
				Status:   True,
				Reason:   RouteTestFailed,
				Category: Critical,
				Message:  fmt.Sprintf("Exposed registry route connection test failed, Response code received: %#v", statusCode),
			})
			return nil
		}
	}
	return nil
}

func checkRegistryConnection(cluster *migapi.MigCluster, kclient k8sclient.Client) (int, error, error) {
	var statusCode int

	url := "https://" + cluster.Spec.ExposedRegistryPath + "/v2/"
	restConfig, err := cluster.BuildRestConfig(kclient)
	token := restConfig.BearerToken
	// Construct transport using default values from http lib
	defaultTransport := http.DefaultTransport.(*http.Transport)
	transport := &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return statusCode, nil, err
	}

	req.Header.Set("Authorization", "bearer "+token)
	res, regErr := client.Do(req)
	if regErr == nil && res != nil {
		statusCode = res.StatusCode
	}
	return statusCode, regErr, nil
}

func (r *ReconcileMigCluster) validateSaTokenPrivileges(ctx context.Context, cluster *migapi.MigCluster) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateSaTokenPrivileges")
		defer span.Finish()
	}

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
			Message:  fmt.Sprintf("The `saToken` has insufficient privileges."),
		})
	}
	return nil
}

// validate operator version.
func (r ReconcileMigCluster) validateOperatorVersionMatchesHost(ctx context.Context, cluster *migapi.MigCluster) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateOperatorVersionMatchesHost")
		defer span.Finish()
	}

	if cluster.Spec.IsHostCluster {
		return nil
	}
	if cluster.Status.HasCriticalCondition() {
		return nil
	}

	clusterClient, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	clusterOperatorVersion, err := cluster.GetOperatorVersion(clusterClient)
	if clusterOperatorVersion == "" {

		cluster.Status.SetCondition(migapi.Condition{
			Type:     ClusterOperatorVersionNotFound,
			Status:   True,
			Reason:   VersionNotFound,
			Category: Warn,
			Message: fmt.Sprintf(`MTC operator version lookup failed on cluster %s. Key %s not found in configmap %s/%s. `+
				`Ensure that all clusters are running the same MTC Operator version.`,
				cluster.ObjectMeta.Name, migapi.OperatorVersionKey, migapi.VeleroNamespace, migapi.ClusterConfigMapName),
		})

		return nil
	}

	hostOperatorVersion, err := cluster.GetOperatorVersion(r)
	if hostOperatorVersion == "" {

		cluster.Status.SetCondition(migapi.Condition{
			Type:     ClusterOperatorVersionNotFound,
			Status:   True,
			Reason:   VersionNotFound,
			Category: Warn,
			Message: fmt.Sprintf(`MTC operator version lookup failed on host cluster. Key %s not found in configmap %s/%s. `+
				`Ensure that all clusters are running the same MTC Operator version.`,
				migapi.OperatorVersionKey, migapi.VeleroNamespace, migapi.ClusterConfigMapName),
		})

		return nil
	}

	operatorVersionMatchesHost := clusterOperatorVersion == hostOperatorVersion

	if !operatorVersionMatchesHost {
		cluster.Status.SetCondition(migapi.Condition{
			Type:     OperatorVersionMismatch,
			Status:   True,
			Reason:   VersionCheckFailed,
			Category: Warn,
			Message:  fmt.Sprintf("This cluster is running a different version of the Migration Toolkit for Containers operator than the host cluster. Migrating to or from this cluster might result in a failed migration and data loss. Make sure all clusters are running the same version of the operator before attempting a migration."),
		})
	}

	return nil
}
