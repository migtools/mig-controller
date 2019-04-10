package migcluster

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Types
const (
	Ready = "Ready"
)

// Reasons
const ()

// Statuses
const (
	True  = "True"
	False = "False"
)

// Messages
const (
	ReadyMessage = "The cluster is ready."
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigCluster) validate(assetCollection *migapi.MigCluster) (error, int) {
	totalSet := 0
	var err error

	// Apply changes
	err = r.Update(context.TODO(), assetCollection)
	if err != nil {
		return err, 0
	}

	return err, totalSet
}
