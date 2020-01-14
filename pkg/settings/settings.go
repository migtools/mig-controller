package settings

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//
// Manager roles.
const (
	// Role environment variable.
	Role = "ROLE"
	// CAM role.
	// Only migration controllers should be loaded.
	CamRole = "cam"
	// Discovery role.
	// Only the discovery should be loaded.
	DiscoveryRole = "discovery"
)

// Global
var Settings = _Settings{}

// Settings
//   Plan: Plan settings.
type _Settings struct {
	Discovery
	Plan
	Roles map[string]bool
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
			case CamRole, DiscoveryRole:
				r.Roles[role] = true
			default:
				list := strings.Join([]string{CamRole, DiscoveryRole}, "|")
				return errors.New(
					fmt.Sprintf(
						"%s must be (%s)",
						Role,
						list))
			}
		}
	} else {
		r.Roles[DiscoveryRole] = true
		r.Roles[CamRole] = true
	}

	return nil
}

//
// Test manager role.
func (r *_Settings) HasRole(name string) bool {
	_, found := r.Roles[name]
	return found
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
