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

package directimagestreammigration

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
	MissingSourceClusterRegistryPath      = "MissingSourceClusterRegistryPath"
	MissingDestinationClusterRegistryPath = "MissingDestinationClusterRegistryPath"
	SourceClusterNotReady                 = "SourceClusterNotReady"
	DestinationClusterNotReady            = "DestinationClusterNotReady"
	InvalidImageStreamRef                 = "InvalidImageStreamRef"
	InvalidImageStream                    = "InvalidImageStream"
	NsNotFoundOnDestinationCluster        = "NamespaceNotFoundOnDestinationCluster"
)

// Validate the image migration resource
func (r ReconcileDirectImageStreamMigration) validate(imageStreamMigration *migapi.DirectImageStreamMigration) error {
	err := r.validateSrcCluster(imageStreamMigration)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateDestCluster(imageStreamMigration)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateImageStream(imageStreamMigration)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateDestNamespace(imageStreamMigration)
	if err != nil {
		return liberr.Wrap(err)
	}
	// imagestream validation?
	return nil
}

func (r ReconcileDirectImageStreamMigration) validateSrcCluster(imageStreamMigration *migapi.DirectImageStreamMigration) error {
	ref := imageStreamMigration.Spec.SrcMigClusterRef

	// Not Set
	if !migref.RefSet(ref) {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
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
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   migapi.True,
			Reason:   migapi.NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("spec.srcMigClusterRef %s must reference a valid `MigCluster`",
				path.Join(imageStreamMigration.Spec.SrcMigClusterRef.Namespace, imageStreamMigration.Spec.SrcMigClusterRef.Name)),
		})
		return nil
	}

	// Not ready
	if !cluster.Status.IsReady() {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterNotReady,
			Status:   migapi.True,
			Reason:   migapi.NotReady,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The source cluster %s is not ready",
				path.Join(imageStreamMigration.Spec.SrcMigClusterRef.Namespace, imageStreamMigration.Spec.SrcMigClusterRef.Name)),
		})
	}
	// Exposed registry path
	registryPath, err := cluster.GetRegistryPath(r)
	if err != nil || registryPath == "" {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     MissingSourceClusterRegistryPath,
			Status:   migapi.True,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The source cluster %s is missing an exposed registry path",
				path.Join(imageStreamMigration.Spec.SrcMigClusterRef.Namespace, imageStreamMigration.Spec.SrcMigClusterRef.Name)),
		})
	}
	return nil
}

func (r ReconcileDirectImageStreamMigration) validateDestCluster(imageStreamMigration *migapi.DirectImageStreamMigration) error {
	ref := imageStreamMigration.Spec.DestMigClusterRef

	if !migref.RefSet(ref) {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   migapi.True,
			Reason:   migapi.NotSet,
			Category: migapi.Critical,
			Message:  "spec.destMigClusterRef must reference name and namespace for a valid `MigCluster`",
		})
		return nil
	}

	// Check if clusters are unique
	if reflect.DeepEqual(ref, imageStreamMigration.Spec.SrcMigClusterRef) {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationCluster,
			Status:   migapi.True,
			Reason:   migapi.NotDistinct,
			Category: migapi.Critical,
			Message:  "directImageStreamMigration.srcMigClusterRef and directImageStreamMigration.destMigClusterRef must reference different clusters",
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Not found
	if cluster == nil {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   migapi.True,
			Reason:   migapi.NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("spec.destMigClusterRef %s must reference a valid `MigCluster`",
				path.Join(imageStreamMigration.Spec.DestMigClusterRef.Namespace, imageStreamMigration.Spec.DestMigClusterRef.Name)),
		})
		return nil
	}

	// Not ready
	if !cluster.Status.IsReady() {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterNotReady,
			Status:   migapi.True,
			Reason:   migapi.NotReady,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The destination cluster %s is not ready",
				path.Join(imageStreamMigration.Spec.DestMigClusterRef.Namespace, imageStreamMigration.Spec.DestMigClusterRef.Name)),
		})
	}
	// Exposed registry path
	registryPath, err := cluster.GetRegistryPath(r)
	if err != nil || registryPath == "" {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     MissingDestinationClusterRegistryPath,
			Status:   migapi.True,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The destination cluster %s is missing an exposed registry path",
				path.Join(imageStreamMigration.Spec.DestMigClusterRef.Namespace, imageStreamMigration.Spec.DestMigClusterRef.Name)),
		})
	}
	return nil
}

func (r ReconcileDirectImageStreamMigration) validateImageStream(imageStreamMigration *migapi.DirectImageStreamMigration) error {
	ref := imageStreamMigration.Spec.ImageStreamRef

	if !migref.RefSet(ref) {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidImageStreamRef,
			Status:   migapi.True,
			Reason:   migapi.NotSet,
			Category: migapi.Critical,
			Message:  "spec.imageStreamRef must reference name and namespace for a valid `ImageStream`",
		})
		return nil
	}

	cluster, err := imageStreamMigration.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}

	is, err := migapi.GetImageStream(client, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Not found
	if is == nil {
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     InvalidImageStream,
			Status:   migapi.True,
			Reason:   migapi.NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("spec.imageStreamRef %s must reference a valid `ImageStream`",
				path.Join(imageStreamMigration.Spec.ImageStreamRef.Namespace, imageStreamMigration.Spec.ImageStreamRef.Name)),
		})
		return nil
	}
	return nil
}

// Validate required namespaces on the source cluster.
// Returns error and the total error conditions set.
func (r ReconcileDirectImageStreamMigration) validateDestNamespace(imageStreamMigration *migapi.DirectImageStreamMigration) error {
	cluster, err := imageStreamMigration.GetDestinationCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	ns := kapi.Namespace{}
	nsName := imageStreamMigration.GetDestinationNamespace()
	err = client.Get(context.TODO(), types.NamespacedName{Name: nsName}, &ns)
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		imageStreamMigration.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnDestinationCluster,
			Status:   migapi.True,
			Reason:   migapi.NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("Namespace %s not found on the destination cluster %s",
				nsName,
				path.Join(imageStreamMigration.Spec.DestMigClusterRef.Namespace, imageStreamMigration.Spec.DestMigClusterRef.Name)),
		})
		return nil
	}
	return nil
}
