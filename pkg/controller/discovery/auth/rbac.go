package auth

import (
	"context"
	"database/sql"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"github.com/fusor/mig-controller/pkg/logging"
	"github.com/fusor/mig-controller/pkg/settings"
	"k8s.io/api/authentication/v1beta1"
	rbac "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

// Application settings.
var Settings = &settings.Settings

// Shared logger.
var Log *logging.Logger

//
// Special Users.
const (
	KubeAdmin = "kube:admin"
)

var (
	AllowUsers = map[string]bool{
		KubeAdmin: true,
	}
)

//
// k8s Resources.
const (
	ALL       = "*"
	Namespace = "namespaces"
	PV        = "persistentvolumes"
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
// RBAC request.
type Request struct {
	// The k8s API resource.
	Resources []string
	// The namespace.
	Namespace string
	// Verbs
	Verbs []string
	// Matrix of expand the Resources and Verbs
	matrix Matrix
}

//
// Expand the Resources and Verbs into the `matrix`.
func (r *Request) expand() {
	r.matrix = Matrix{}
	for _, resource := range r.Resources {
		for _, verb := range r.Verbs {
			r.matrix = append(r.matrix, MxItem{resource: resource, verb: verb})
		}
	}
}

//
// Apply the rule to the matrix.
func (r *Request) apply(rule *rbac.PolicyRule) {
	matrix := Matrix{}
	for _, resource := range rule.Resources {
		for _, verb := range rule.Verbs {
			matrix = append(matrix, MxItem{resource: resource, verb: verb})
		}
	}
	for i := range r.matrix {
		for _, m2 := range matrix {
			m := &r.matrix[i]
			if !m.matched {
				m.match(&m2)
			}
		}
	}
}

//
// Return `true` when all of the matrix items have been matched.
func (r *Request) satisfied() bool {
	for _, m := range r.matrix {
		if !m.matched {
			return false
		}
	}

	return true
}

//
// The matrix is a de-normalized set of Resources and verbs.
type Matrix = []MxItem

//
// A matrix item.
type MxItem struct {
	resource string
	verb     string
	matched  bool
}

//
// Match another matrix item.
func (m *MxItem) match(m2 *MxItem) {
	if m.resource == ANY || m2.resource == ANY || m.resource == m2.resource {
		if m.verb == ANY || m2.verb == ANY || m.verb == m2.verb {
			m.matched = true
		}
	}
}

//
// RBAC
type RBAC struct {
	Client client.Client
	// Database
	Db *sql.DB
	// Cluster
	Cluster *model.Cluster
	// A Bearer token.
	Token string
	// The ServiceAccount for the token.
	sa types.NamespacedName
	// The User for the token.
	user string
	// The user group membership.
	groups []string
	// RoleBindings for token.
	roleBindings []*model.RoleBinding
	// The token has been authenticated.
	authenticated bool
	// The role-bindings have been loaded.
	loaded bool
}

//
// Allow request.
func (r *RBAC) Allow(request *Request) (bool, error) {
	if r.Token == "" && Settings.Discovery.AuthOptional {
		return true, nil
	}
	err := r.load()
	if err != nil {
		return false, nil
	}
	if !r.authenticated {
		return false, nil
	}
	if _, found := AllowUsers[r.user]; found {
		return true, nil
	}
	request.expand()
	for _, rb := range r.roleBindings {
		role, err := rb.GetRole(r.Db)
		if err != nil {
			continue
		}
		if rb.Namespace == "" || rb.Namespace == request.Namespace {
			if r.matchRules(request, role) {
				return true, nil
			}
		}
	}

	return false, nil
}

//
// Match the rule.
func (r *RBAC) matchRules(request *Request, role *model.Role) bool {
	rules := role.DecodeRules()
	for _, rule := range rules {
		request.apply(&rule)
		if request.satisfied() {
			return true
		}
	}

	return false
}

//
// Resolve the token to a User or SA.
// Load the associated `RoleBindings`.
func (r *RBAC) load() error {
	if r.loaded {
		return nil
	}
	err := r.authenticate()
	if err != nil {
		Log.Trace(err)
		return err
	}
	err = r.buildRoleBindings()
	if err != nil {
		Log.Trace(err)
		return err
	}

	r.loaded = true

	return nil
}

//
// Authenticate the bearer token.
// Set the user|sa and groups.
func (r *RBAC) authenticate() error {
	mark := time.Now()
	tr := v1beta1.TokenReview{
		Spec: v1beta1.TokenReviewSpec{
			Token: r.Token,
		},
	}
	err := r.Client.Create(context.TODO(), &tr)
	if err != nil {
		Log.Trace(err)
		return err
	}
	if !tr.Status.Authenticated {
		return nil
	}
	r.authenticated = true
	user := tr.Status.User
	r.groups = user.Groups
	name := strings.Split(user.Username, ":")
	if len(name) == 4 {
		if name[0] == "system" && name[1] == "serviceaccount" {
			r.sa.Namespace = name[2]
			r.sa.Name = name[3]
		}
	} else {
		r.user = user.Username
	}

	Log.Info("RBAC (authenticate):", "duration", time.Since(mark))

	return nil
}

func (r *RBAC) buildRoleBindings() error {
	var subject model.Subject
	var err error
	if !r.authenticated {
		return nil
	}
	if r.user != "" {
		if _, found := AllowUsers[r.user]; found {
			return nil
		}
		subject = model.Subject{
			Kind: model.SubjectUser,
			Name: r.user,
		}
	}
	if r.sa.Name != "" {
		subject = model.Subject{
			Kind:      model.SubjectSa,
			Namespace: r.sa.Namespace,
			Name:      r.sa.Name,
		}
	}
	r.roleBindings, err = r.Cluster.RoleBindingListBySubject(r.Db, subject)
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, group := range r.groups {
		subject = model.Subject{
			Kind: model.SubjectGroup,
			Name: group,
		}
		roleBindings, err := r.Cluster.RoleBindingListBySubject(r.Db, subject)
		if err != nil {
			Log.Trace(err)
			return err
		}
		for _, rb := range roleBindings {
			r.roleBindings = append(r.roleBindings, rb)
		}
	}

	return nil
}
