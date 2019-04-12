package migassetcollection

import (
	"context"
	"fmt"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"strings"
)

// Types
const (
	EmptyCollection    = "EmptyCollection"
	NamespacesNotFound = "NamespacesNotFound"
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
	ReadyMessage             = "The asset-collection is ready."
	NamespaceNotFoundMessage = "The following `namespaces` [%s] not found on the source cluster."
	EmptyCollectionMessage   = "The `namespaces` list may not be empty."
)

// Validate the asset collection resource.
// Returns error and the total error conditions set.
func (r ReconcileMigAssetCollection) validate(assetCollection *migapi.MigAssetCollection) (error, int) {
	totalSet := 0

	// Empty collection
	err, nSet := r.validateEmpty(assetCollection)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Validate listed assets
	err, nSet = r.validateAssets(assetCollection)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Ready
	assetCollection.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes
	err = r.Update(context.TODO(), assetCollection)
	if err != nil {
		return err, 0
	}

	return err, totalSet
}

func (r ReconcileMigAssetCollection) validateEmpty(assetCollection *migapi.MigAssetCollection) (error, int) {
	if len(assetCollection.Spec.Namespaces) == 0 {
		assetCollection.Status.SetCondition(migapi.Condition{
			Type:    EmptyCollection,
			Status:  True,
			Message: EmptyCollectionMessage,
		})
		return nil, 1
	} else {
		assetCollection.Status.DeleteCondition(EmptyCollection)
	}

	return nil, 0
}

func (r ReconcileMigAssetCollection) validateAssets(assetCollection *migapi.MigAssetCollection) (error, int) {
	notFound := make([]string, 0)
	ns := kapi.Namespace{}
	for _, name := range assetCollection.Spec.Namespaces {
		key := types.NamespacedName{Name: name}
		err := r.Get(context.TODO(), key, &ns) // TODO: query source cluster instead.
		if err == nil {
			continue
		}
		if errors.IsNotFound(err) {
			notFound = append(notFound, name)
		} else {
			return err, 0
		}
	}

	if len(notFound) > 0 {
		message := fmt.Sprintf(NamespaceNotFoundMessage, strings.Join(notFound, ", "))
		assetCollection.Status.SetCondition(migapi.Condition{
			Type:    NamespacesNotFound,
			Status:  True,
			Reason:  NotFound,
			Message: message,
		})
		return nil, 1
	} else {
		assetCollection.Status.DeleteCondition(NamespacesNotFound)
	}

	return nil, 0
}
