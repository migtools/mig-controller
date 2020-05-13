package settings

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	// Role environment variable.
	EnvRole = "ROLE"
	// Role names
	ClusterRole   = "cluster"
	PlanRole      = "plan"
	MigrationRole = "migration"
	StorageRole   = "storage"
	DiscoveryRole = "discovery"
	TokenRole     = "token"
)

// Role settings.
type Role struct {
	enabled map[string]bool
}

//
// Load the manager role.
// The default is ALL roles.
func (r *Role) Load() error {
	r.enabled = map[string]bool{}
	if s, found := os.LookupEnv(EnvRole); found {
		for _, role := range strings.Split(s, ",") {
			role = strings.ToLower(strings.TrimSpace(role))
			switch role {
			case ClusterRole, MigrationRole, PlanRole, StorageRole, DiscoveryRole, TokenRole:
				r.enabled[role] = true
			default:
				list := strings.Join([]string{ClusterRole, MigrationRole, PlanRole, StorageRole, DiscoveryRole, TokenRole}, "|")
				return errors.New(
					fmt.Sprintf(
						"%s must be (%s)",
						EnvRole,
						list))
			}
		}
	} else {
		r.enabled[DiscoveryRole] = true
		r.enabled[ClusterRole] = true
		r.enabled[PlanRole] = true
		r.enabled[MigrationRole] = true
		r.enabled[StorageRole] = true
		r.enabled[TokenRole] = true
	}

	return nil
}

// Return whether the given role is enabled.
func (r *Role) Enabled(name string) bool {
	_, found := r.enabled[name]
	return found
}
