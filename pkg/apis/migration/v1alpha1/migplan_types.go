/*
Copyright 2019 Red Hat Inc.

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

package v1alpha1

import (
	"fmt"

	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MigPlanSpec defines the desired state of MigPlan
type MigPlanSpec struct {
	SrcMigClusterRef      *kapi.ObjectReference `json:"srcMigClusterRef,omitempty"`
	DestMigClusterRef     *kapi.ObjectReference `json:"destMigClusterRef,omitempty"`
	MigStorageRef         *kapi.ObjectReference `json:"migStorageRef,omitempty"`
	MigAssetCollectionRef *kapi.ObjectReference `json:"migAssetCollectionRef,omitempty"`
}

// MigPlanStatus defines the observed state of MigPlan
type MigPlanStatus struct {
	Conditions
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigPlan is the Schema for the migplans API
// +k8s:openapi-gen=true
type MigPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigPlanSpec   `json:"spec,omitempty"`
	Status MigPlanStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigPlanList contains a list of MigPlan
type MigPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigPlan{}, &MigPlanList{})
}

// GetSourceCluster - Get the referenced source cluster.
// Returns `nil` when the reference cannot be resolved.
func (r *MigPlan) GetSourceCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.SrcMigClusterRef)
}

// GetDestinationCluster - Get the referenced destination cluster.
// Returns `nil` when the reference cannot be resolved.
func (r *MigPlan) GetDestinationCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.DestMigClusterRef)
}

// GetStorage - Get the referenced storage..
// Returns `nil` when the reference cannot be resolved.
func (r *MigPlan) GetStorage(client k8sclient.Client) (*MigStorage, error) {
	return GetStorage(client, r.Spec.MigStorageRef)
}

// GetAssetCollection - Get the referenced asset-collection.
// Returns `nil` when the reference cannot be resolved.
func (r *MigPlan) GetAssetCollection(client k8sclient.Client) (*MigAssetCollection, error) {
	return GetAssetCollection(client, r.Spec.MigAssetCollectionRef)
}

// PlanRefResources holds structs for the resources MigPlans reference.
// This data is passed to functions within MigStage and MigMigration reconcile.
type PlanRefResources struct {
	MigPlan        *MigPlan
	MigAssets      *MigAssetCollection
	MigStorage     *MigStorage
	SrcMigCluster  *MigCluster
	DestMigCluster *MigCluster

	SrcBackup   *velerov1.Backup
	DestRestore *velerov1.Restore
}

// GetRefResources gets referenced resources from a MigPlan and loads them into a ReferencedResources struct
func (r *MigPlan) GetRefResources(client k8sclient.Client, logPrefix string) (*PlanRefResources, error) {
	resources := &PlanRefResources{}

	// MigPlan
	resources.MigPlan = r

	// MigAssetCollection
	migAssets, err := r.GetAssetCollection(client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET MigAssetCollection referenced by MigPlan [%s/%s]",
			logPrefix, r.Namespace, r.Name))
		return nil, err
	}
	resources.MigAssets = migAssets

	// MigStorage
	storage, err := r.GetStorage(client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET MigAssetCollection referenced by MigStorage [%s/%s]",
			logPrefix, r.Namespace, r.Name))
		return nil, err
	}
	resources.MigStorage = storage

	// SrcMigCluster
	srcMigCluster, err := r.GetSourceCluster(client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET SrcMigCluster referenced by MigPlan [%s/%s]",
			logPrefix, r.Namespace, r.Name))
		return nil, err
	}
	resources.SrcMigCluster = srcMigCluster

	// DestMigCluster
	destMigCluster, err := r.GetDestinationCluster(client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET DestMigCluster referenced by MigPlan [%s/%s]",
			logPrefix, r.Namespace, r.Name))
		return nil, err
	}
	resources.DestMigCluster = destMigCluster

	return resources, nil
}
