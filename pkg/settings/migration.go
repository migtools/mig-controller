package settings

import (
	"os"
	"strings"
)

// Environment variables.
const (
	ClusterScopedBlacklist = "CLUSTER_SCOPED_BLACKLIST"
)

/*
	This is the list of resources which are automatically included in a migration
	if a namespace scoped resource depends on it:

	"persistentvolumes",
	"securitycontextconstraints",
	"clusterroles",
	"clusterrolebindings",
	"clusterresourcequotas",
*/
// Defaults
var DefaultBlacklist = []string{}

// Migration settings.
//   ClusterScopedBlacklist: Blacklist of cluster-scoped resources a tenant cannot migrate
type Migration struct {
	ClusterScopedBlacklist []string
}

// Load settings.
func (r *Migration) Load() error {
	var err error
	r.ClusterScopedBlacklist = getClusterScopedBlacklist(ClusterScopedBlacklist, DefaultBlacklist)
	return err
}

// Get blacklist from env vars
func getClusterScopedBlacklist(name string, def []string) []string {
	blacklist := def
	if s, found := os.LookupEnv(name); found {
		blacklist = strings.Split(s, ",")
	}
	return blacklist
}
