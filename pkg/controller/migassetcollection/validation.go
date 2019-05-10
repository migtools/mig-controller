package migassetcollection

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Types
const (
	EmptyCollection = "EmptyCollection"
)

// Reasons
const (
	NotFound = "NotFound"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage           = "The asset-collection is ready."
	EmptyCollectionMessage = "The `namespaces` list may not be empty."
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigAssetCollection) validate(assetCollection *migapi.MigAssetCollection) (int, error) {
	totalSet := 0

	// Empty collection
	nSet, err := r.validateEmpty(assetCollection)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Ready
	assetCollection.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes
	assetCollection.Status.CommitConditions()
	err = r.Update(context.TODO(), assetCollection)
	if err != nil {
		return 0, err
	}

	return totalSet, err
}

func (r ReconcileMigAssetCollection) validateEmpty(assetCollection *migapi.MigAssetCollection) (int, error) {
	if len(assetCollection.Spec.Namespaces) == 0 {
		assetCollection.Status.SetCondition(migapi.Condition{
			Type:    EmptyCollection,
			Status:  True,
			Message: EmptyCollectionMessage,
		})
		return 1, nil
	}

	return 0, nil
}
