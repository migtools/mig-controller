package settings

import (
	"errors"
	"os"
	"strconv"
)

// Global
var Settings = _Settings{}

// Settings
//   Plan: Plan settings.
type _Settings struct {
	Discovery
	Plan
	Proxy
	Role
}

// Load settings.
func (r *_Settings) Load() error {
	err := r.Discovery.Load()
	if err != nil {
		return err
	}
	err = r.Plan.Load()
	if err != nil {
		return err
	}
	err = r.Proxy.Load()
	if err != nil {
		return err
	}
	err = r.Role.Load()
	if err != nil {
		return err
	}

	return nil
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
