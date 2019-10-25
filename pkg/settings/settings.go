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
	Plan Plan
}

// Load settings.
func (r *_Settings) Load() error {
	err := r.Plan.Load()
	return err
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
