/*
Copyright 2020 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package directimagemigration

import (
	"context"
	"fmt"
	"path"
	"reflect"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// Types
const (
	InvalidSourceClusterRef               = "InvalidSourceClusterRef"
	InvalidDestinationClusterRef          = "InvalidDestinationClusterRef"
	InvalidDestinationCluster             = "InvalidDestinationCluster"
	SourceClusterNotReady                 = "SourceClusterNotReady"
	DestinationClusterNotReady            = "DestinationClusterNotReady"
	MissingSourceClusterRegistryPath      = "MissingSourceClusterRegistryPath"
	MissingDestinationClusterRegistryPath = "MissingDestinationClusterRegistryPath"
	NsListEmpty                           = "NamespaceListEmpty"
	NsNotFoundOnSourceCluster             = "NamespaceNotFoundOnSourceCluster"
)

// Validate the image migration resource
func (r ReconcileDirectImageMigration) validate(imageMigration *migapi.DirectImageMigration) error {
	err := r.validateSrcCluster(imageMigration)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateDestCluster(imageMigration)
	if err != nil {
		return liberr.Wrap(err)
	}
	// Migrated namespaces.
	err = r.validateNamespaces(imageMigration)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (r ReconcileDirectImageMigration) validateSrcCluster(imageMigration *migapi.DirectImageMigration) error {
	ref := imageMigration.Spec.SrcMigClusterRef

	// Not Set
	if !migref.RefSet(ref) {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   migapi.True,
			Reason:   migapi.NotSet,
			Category: migapi.Critical,
			Message:  "spec.srcMigClusterRef must reference name and namespace for a valid `MigCluster`",
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Not found
	if cluster == nil {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   migapi.True,
			Reason:   migapi.NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("spec.srcMigClusterRef %s must reference a valid `MigCluster`",
				path.Join(imageMigration.Spec.SrcMigClusterRef.Namespace, imageMigration.Spec.SrcMigClusterRef.Name)),
		})
		return nil
	}

	// Not ready
	if !cluster.Status.IsReady() {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterNotReady,
			Status:   migapi.True,
			Reason:   migapi.NotReady,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The source cluster %s is not ready",
				path.Join(imageMigration.Spec.SrcMigClusterRef.Namespace, imageMigration.Spec.SrcMigClusterRef.Name)),
		})
	}
	// Exposed registry path
	registryPath, err := cluster.GetRegistryPath(r)
	if err != nil || registryPath == "" {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     MissingSourceClusterRegistryPath,
			Status:   migapi.True,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The source cluster %s is missing an exposed registry path",
				path.Join(imageMigration.Spec.SrcMigClusterRef.Namespace, imageMigration.Spec.SrcMigClusterRef.Name)),
		})
	}
	return nil
}

func (r ReconcileDirectImageMigration) validateDestCluster(imageMigration *migapi.DirectImageMigration) error {
	ref := imageMigration.Spec.DestMigClusterRef

	if !migref.RefSet(ref) {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   migapi.True,
			Reason:   migapi.NotSet,
			Category: migapi.Critical,
			Message:  "spec.destMigClusterRef must reference name and namespace for a valid `MigCluster`",
		})
		return nil
	}

	// Check if clusters are unique
	if reflect.DeepEqual(ref, imageMigration.Spec.SrcMigClusterRef) {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationCluster,
			Status:   migapi.True,
			Reason:   migapi.NotDistinct,
			Category: migapi.Critical,
			Message:  "directImageMigration.srcMigClusterRef and directImageMigration.destMigClusterRef must reference different clusters",
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Not found
	if cluster == nil {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   migapi.True,
			Reason:   migapi.NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("spec.destMigClusterRef %s must reference a valid `MigCluster`",
				path.Join(imageMigration.Spec.DestMigClusterRef.Namespace, imageMigration.Spec.DestMigClusterRef.Name)),
		})
		return nil
	}

	// Not ready
	if !cluster.Status.IsReady() {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterNotReady,
			Status:   migapi.True,
			Reason:   migapi.NotReady,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The destination cluster %s is not ready",
				path.Join(imageMigration.Spec.DestMigClusterRef.Namespace, imageMigration.Spec.DestMigClusterRef.Name)),
		})
	}
	// Exposed registry path
	registryPath, err := cluster.GetRegistryPath(r)
	if err != nil || registryPath == "" {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     MissingDestinationClusterRegistryPath,
			Status:   migapi.True,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The destination cluster %s is missing an exposed registry path",
				path.Join(imageMigration.Spec.DestMigClusterRef.Namespace, imageMigration.Spec.DestMigClusterRef.Name)),
		})
	}
	return nil
}

// Validate required namespaces on the source cluster.
// Returns error and the total error conditions set.
func (r ReconcileDirectImageMigration) validateNamespaces(imageMigration *migapi.DirectImageMigration) error {
	count := len(imageMigration.Spec.Namespaces)
	if count == 0 {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     NsListEmpty,
			Status:   migapi.True,
			Category: migapi.Critical,
			Message:  "The `namespaces` list may not be empty.",
		})
		return nil
	}
	srcCluster, err := imageMigration.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcCluster == nil || !srcCluster.Status.IsReady() {
		return nil
	}
	srcClient, err := srcCluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	ns := kapi.Namespace{}
	notFound := make([]string, 0)
	for _, nsName := range imageMigration.GetSourceNamespaces() {
		err := srcClient.Get(context.TODO(), types.NamespacedName{Name: nsName}, &ns)
		if err == nil {
			continue
		}
		if k8serror.IsNotFound(err) {
			notFound = append(notFound, nsName)
		} else {
			return liberr.Wrap(err)
		}
	}
	if len(notFound) > 0 {
		imageMigration.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnSourceCluster,
			Status:   migapi.True,
			Reason:   migapi.NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("Namespaces [] not found on the source cluster %s",
				path.Join(imageMigration.Spec.SrcMigClusterRef.Namespace, imageMigration.Spec.SrcMigClusterRef.Name)),
			Items: notFound,
		})
		return nil
	}

	return nil
}
