package auth

import (
	"context"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/logging"
	project "github.com/openshift/api/project/v1"
	auth "k8s.io/api/authentication/v1beta1"
	"k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Shared logger.
var Log *logging.Logger

//
// k8s Resources.
const (
	ALL       = "*"
	Namespace = "namespaces"
	PV        = "persistentvolumes"
	PVC       = "persistentvolumeclaims"
	Service   = "services"
	Pod       = "pods"
	PodLog    = "pods/log"
)

//
// Verbs
const (
	ANY    = "*"
	LIST   = "list"
	GET    = "get"
	CREATE = "create"
	DELETE = "delete"
	PATCH  = "patch"
	UPDATE = "update"
)

//
// SAR request.
type Review = v1.ResourceAttributes

//
// RBAC authorization provider.
type Provider interface {
	// Authenticate the token.
	Authenticate() (bool, error)
	// Get whether the token was authenticated.
	Authenticated() bool
	// Determine of actions specified are authorized
	// for the token.
	Allow(*Review) (bool, error)
	// Determine if the all of the actions
	// specified in the matrix are authorized for
	// the token.
	AllowMatrix(*Matrix) (bool, error)
	// Get the user name.
	User() string
}

//
// Review Matrix.
type Matrix struct {
	// API group.
	Group string
	// Namespace
	Namespace string
	// Resource name.
	Name string
	// Resources.
	Resources []string
	// Verbs.
	Verbs []string
}

//
// Provides RBAC authorization.
type RBAC struct {
	client.Client
	// Cluster
	Cluster *migapi.MigCluster
	// The subject bearer token.
	Token string
	// Token review status.
	tokenStatus auth.TokenReviewStatus
	// Token authenticated.
	authenticated *bool
	// Project-based authorization.
	pbac *PBAC
}

//
// Allow/deny the subject review request.
// Special (optimized) steps for namespaces:
//   1. Populate `allowedProject`.
//   2. Match namespace in `allowedProject`.
//   3. Match `*` in `allowedProject`.
//   4. SAR
// The SAR for each namespace is last resort. For large
// clusters this will be very slow.
func (r *RBAC) Allow(request *Review) (bool, error) {
	authenticated, err := r.Authenticate()
	if err != nil {
		Log.Trace(err)
		return false, err
	}
	if !authenticated {
		return false, nil
	}
	if r.pbac == nil {
		r.pbac = &PBAC{
			Cluster: r.Cluster,
			Token:   r.Token,
		}
	}
	switch request.Resource {
	case Namespace:
		switch request.Verb {
		case LIST, GET:
			if r.pbac.Supported() {
				allowed := r.pbac.Allow(request.Name)
				return allowed, nil
			}
		}
		fallthrough
	default:
		return r.sar(request)
	}
}

//
// Allow all the combinations in the matrix.
// See: Allow() for details.
func (r *RBAC) AllowMatrix(m *Matrix) (bool, error) {
	mAllowed := false
	request := &Review{
		Group:     m.Group,
		Namespace: m.Namespace,
		Name:      m.Name,
	}
	for _, resource := range m.Resources {
		for _, verb := range m.Verbs {
			mAllowed = true
			request.Resource = resource
			request.Verb = verb
			allowed, err := r.Allow(request)
			if err != nil {
				Log.Trace(err)
				return false, err
			}
			if !allowed {
				return false, nil
			}
		}
	}

	return mAllowed, nil
}

//
// Get whether authenticated
func (r *RBAC) Authenticated() bool {
	return r != nil && *r.authenticated
}

//
// Get the user name for the token.
func (r *RBAC) User() string {
	if r.Authenticated() {
		return r.tokenStatus.User.Username
	}

	return ""
}

//
// Get the user ID for the token.
func (r *RBAC) UID() string {
	if r.Authenticated() {
		return r.tokenStatus.User.UID
	}

	return ""
}

//
// Get the user's groups.
func (r *RBAC) Groups() []string {
	if r.Authenticated() {
		return r.tokenStatus.User.Groups
	}

	return []string{}
}

//
// Get the extra fields for the token.
func (r *RBAC) Extra() map[string]v1.ExtraValue {
	extra := map[string]v1.ExtraValue{}
	if r.Authenticated() {
		for k, v := range r.tokenStatus.User.Extra {
			extra[k] = append(
				v1.ExtraValue{},
				v...)
		}
	}

	return extra
}

//
// Do subject access review.
func (r *RBAC) sar(request *Review) (bool, error) {
	sar := v1.SubjectAccessReview{
		Spec: v1.SubjectAccessReviewSpec{
			ResourceAttributes: request,
			Groups:             r.Groups(),
			User:               r.User(),
			UID:                r.UID(),
			Extra:              r.Extra(),
		},
	}
	err := r.Client.Create(context.TODO(), &sar)
	if err != nil {
		Log.Trace(err)
		return false, err
	}

	return sar.Status.Allowed, nil
}

//
// Authenticate the token.
// Build the `subjectClient`.
func (r *RBAC) Authenticate() (bool, error) {
	if r.authenticated != nil {
		return *r.authenticated, nil
	}
	r.authenticated = pointer.BoolPtr(false)
	tr := &auth.TokenReview{
		Spec: auth.TokenReviewSpec{
			Token: r.Token,
		},
	}
	err := r.Client.Create(context.TODO(), tr)
	if err != nil {
		Log.Trace(err)
		return false, err
	}
	if !tr.Status.Authenticated {
		return false, err
	}
	r.tokenStatus = tr.Status
	r.authenticated = pointer.BoolPtr(true)

	return true, nil
}

//
// Provides project-based authorization.
type PBAC struct {
	// Token
	Token string
	// Cluster
	Cluster *migapi.MigCluster
	// Project API supported by the cluster.
	supported bool
	// List of projects.
	projects map[string]bool
}

//
// Project API is supported by the cluster.
func (p *PBAC) Supported() bool {
	p.Load()
	return p.supported
}

//
// The token is allowed access to the specified namespace.
func (p *PBAC) Allow(ns string) bool {
	p.Load()
	_, found := p.projects[ns]
	return found
}

//
// Load projects (as needed).
func (p *PBAC) Load() {
	if p.projects != nil {
		return
	}
	p.projects = map[string]bool{}
	tokenClient, err := p.client()
	if err != nil {
		return
	}
	list := &project.ProjectList{}
	err = tokenClient.List(context.TODO(), nil, list)
	if err != nil {
		return
	}
	p.supported = true
	for _, project := range list.Items {
		p.projects[project.Name] = true
	}
}

//
// Build a client for the token.
func (p *PBAC) client() (client.Client, error) {
	var err error
	restCfg, err := p.Cluster.BuildRestConfigWithToken(p.Token)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	newClient, err := client.New(
		restCfg,
		client.Options{
			Scheme: scheme.Scheme,
		})
	if err != nil {
		if stErr, cast := err.(*errors.StatusError); cast {
			switch stErr.Status().Code {
			case http.StatusUnauthorized,
				http.StatusForbidden:
				return nil, err
			}
		}
		Log.Trace(err)
		return nil, err
	}

	return newClient, nil
}
