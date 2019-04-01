package migstorage

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Types
const (
	// Type
	Ready = "Ready"
)

// Reasons
const ()

// Status
const (
	True  = "True"
	False = "False"
)

// Validate the storage resource.
// Returns error and the total error conditions set.
func (r ReconcileMigStorage) validate(plan *migapi.MigStorage) (error, int) {
	totalSet := 0
	return nil, totalSet
}
