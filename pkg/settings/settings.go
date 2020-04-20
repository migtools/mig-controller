package settings

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

//
// Manager roles.
const (
	// Role environment variable.
	Role          = "ROLE"
	ClusterRole   = "cluster"
	PlanRole      = "plan"
	MigrationRole = "migration"
	StorageRole   = "storage"
	DiscoveryRole = "discovery"
	// Proxy environment variables
	HttpProxy  = "HTTP_PROXY"
	HttpsProxy = "HTTPS_PROXY"
	NoProxy    = "NO_PROXY"
	//
	PrivilegedNamespace = "PRIVILEGED_NAMESPACE"
	SandboxNamespace    = "SANDBOX_NAMESPACE"
)

// Global
var Settings = _Settings{}

// Settings
//   Plan: Plan settings.
type _Settings struct {
	Discovery
	Plan
	Roles               map[string]bool
	ProxyVars           map[string]string
	PrivilegedNamespace string
	SandboxNamespace    string
}

// Load settings.
func (r *_Settings) Load() error {
	err := r.Plan.Load()
	if err != nil {
		return err
	}
	err = r.Discovery.Load()
	if err != nil {
		return err
	}
	err = r.loadRoles()
	if err != nil {
		return err
	}
	err = r.loadProxyVars()
	if err != nil {
		return err
	}

	return nil
}

func (r *_Settings) InSandbox(m v1alpha1.MigResource) bool {
	return m.GetNamespace() == r.SandboxNamespace
}

func (r *_Settings) InPrivileged(m v1alpha1.MigResource) bool {
	return m.GetNamespace() == r.PrivilegedNamespace
}

func (r *_Settings) loadNamespaceVars() error {
	if s, found := os.LookupEnv(SandboxNamespace); found {
		r.SandboxNamespace = s
	}
	if s, found := os.LookupEnv(PrivilegedNamespace); found {
		r.PrivilegedNamespace = s
	}
	return nil
}

func (r *_Settings) loadProxyVars() error {
	r.ProxyVars = map[string]string{}
	if s, found := os.LookupEnv(HttpProxy); found {
		r.ProxyVars[HttpProxy] = s
	}
	if s, found := os.LookupEnv(HttpsProxy); found {
		r.ProxyVars[HttpsProxy] = s
	}
	if s, found := os.LookupEnv(NoProxy); found {
		r.ProxyVars[NoProxy] = s
	}
	return nil
}

//
// Load the manager role.
// The default is ALL roles.
func (r *_Settings) loadRoles() error {
	r.Roles = map[string]bool{}
	if s, found := os.LookupEnv(Role); found {
		for _, role := range strings.Split(s, ",") {
			role = strings.ToLower(strings.TrimSpace(role))
			switch role {
			case ClusterRole, MigrationRole, PlanRole, StorageRole, DiscoveryRole:
				r.Roles[role] = true
			default:
				list := strings.Join([]string{ClusterRole, MigrationRole, PlanRole, StorageRole, DiscoveryRole}, "|")
				return errors.New(
					fmt.Sprintf(
						"%s must be (%s)",
						Role,
						list))
			}
		}
	} else {
		r.Roles[DiscoveryRole] = true
		r.Roles[ClusterRole] = true
		r.Roles[PlanRole] = true
		r.Roles[MigrationRole] = true
		r.Roles[StorageRole] = true
	}

	return nil
}

//
// Test manager role.
func (r *_Settings) HasRole(name string) bool {
	_, found := r.Roles[name]
	return found
}

// Get Proxy Var
func (r *_Settings) HasProxyVar(name string) (bool, string) {
	env, found := r.ProxyVars[name]
	return found, env
}

// Get positive integer limit from the environment
// using the specified variable name and default.
func getEnvLimit(name string, def int) (int, error) {
	limit := 0
	if s, found := os.LookupEnv(name); found {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, errors.New(name + " must be an integer")
		}
		if n < 1 {
			return 0, errors.New(name + " must be >= 1")
		}
		limit = n
	} else {
		limit = def
	}

	return limit, nil
}

// Get boolean.
func getEnvBool(name string, def bool) bool {
	boolean := def
	if s, found := os.LookupEnv(name); found {
		parsed, err := strconv.ParseBool(s)
		if err == nil {
			boolean = parsed
		}
	}

	return boolean
}
