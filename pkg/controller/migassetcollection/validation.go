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
func (r ReconcileMigAssetCollection) validate(assetCollection *migapi.MigAssetCollection) error {
	assetCollection.Status.BeginStagingConditions()

	// Empty collection
	err := r.validateEmpty(assetCollection)
	if err != nil {
		return err
	}

	// Apply changes
	assetCollection.Status.EndStagingConditions()
	err = r.Update(context.TODO(), assetCollection)
	if err != nil {
		return err
	}

	// Ready
	assetCollection.Status.SetReady(
		!assetCollection.Status.HasBlockerCondition(),
		ReadyMessage)

	// Apply changes.
	assetCollection.Status.EndStagingConditions()
	err = r.Update(context.TODO(), assetCollection)
	if err != nil {
		return err
	}

	return nil
}

// Validate that the namespaces list is not empty.
func (r ReconcileMigAssetCollection) validateEmpty(assetCollection *migapi.MigAssetCollection) error {
	if len(assetCollection.Spec.Namespaces) == 0 {
		assetCollection.Status.SetCondition(migapi.Condition{
			Type:     EmptyCollection,
			Status:   True,
			Category: migapi.Error,
			Message:  EmptyCollectionMessage,
		})
		return nil
	}

	return nil
}
