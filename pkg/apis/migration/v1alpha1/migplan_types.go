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
	"errors"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MigPlanSpec defines the desired state of MigPlan
type MigPlanSpec struct {
	PersistentVolumes
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

// Resources referenced by the plan.
// Contains all of the fetched referenced resources.
type PlanResources struct {
	MigPlan        *MigPlan
	MigAssets      *MigAssetCollection
	MigStorage     *MigStorage
	SrcMigCluster  *MigCluster
	DestMigCluster *MigCluster
}

// GetRefResources gets referenced resources from a MigPlan.
func (r *MigPlan) GetRefResources(client k8sclient.Client) (*PlanResources, error) {
	// MigAssetCollection
	migAssets, err := r.GetAssetCollection(client)
	if err != nil {
		return nil, err
	}
	if migAssets == nil {
		return nil, errors.New("asset-collection not found")
	}

	// MigStorage
	storage, err := r.GetStorage(client)
	if err != nil {
		return nil, err
	}
	if storage == nil {
		return nil, errors.New("storage not found")
	}

	// SrcMigCluster
	srcMigCluster, err := r.GetSourceCluster(client)
	if err != nil {
		return nil, err
	}
	if srcMigCluster == nil {
		return nil, errors.New("source cluster not found")
	}

	// DestMigCluster
	destMigCluster, err := r.GetDestinationCluster(client)
	if err != nil {
		return nil, err
	}
	if destMigCluster == nil {
		return nil, errors.New("destination cluster not found")
	}

	resources := &PlanResources{
		MigPlan:        r,
		MigAssets:      migAssets,
		MigStorage:     storage,
		SrcMigCluster:  srcMigCluster,
		DestMigCluster: destMigCluster,
	}

	return resources, nil
}

// PV Actions.
const (
	PvMoveAction = "move"
	PvCopyAction = "copy"
)

// Name - The PV name.
// StorageClass - The PV storage class name.
// Action - The PV migration action (move|copy)
// staged - A PV has been explicitly added/updated.
type PersistentVolume struct {
	Name         string `json:"name,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
	Action       string `json:"action,omitempty"`
	staged       bool
}

// Update the PV with another.
func (r *PersistentVolume) Update(pv PersistentVolume) {
	r.StorageClass = pv.StorageClass
	r.staged = true
}

// Collection of PVs
// List - The collection of PVs.
// index - List index.
// --------
// Example:
// plan.Spec.BeginPvStaging()
// plan.Spec.AddPv(pvA)
// plan.Spec.AddPv(pvB)
// plan.Spec.AddPv(pvC)
// plan.Spec.EndPvStaging()
//
type PersistentVolumes struct {
	List    []PersistentVolume `json:"persistentVolumes"`
	index   map[string]int
	staging bool
}

// Allocate collections.
func (r *PersistentVolumes) init() {
	if r.List == nil {
		r.List = []PersistentVolume{}
	}
	if r.index == nil {
		r.buildIndex()
	}
}

// Build the index.
func (r *PersistentVolumes) buildIndex() {
	r.index = map[string]int{}
	for i, pv := range r.List {
		r.index[pv.Name] = i
	}
}

// Begin staging PVs.
func (r *PersistentVolumes) BeginPvStaging() {
	r.init()
	r.staging = true
	for i := range r.List {
		pv := &r.List[i]
		pv.staged = false
	}
}

// End staging PVs and delete un-staged PVs from the list.
func (r *PersistentVolumes) EndPvStaging() {
	r.init()
	r.staging = false
	kept := []PersistentVolume{}
	for _, pv := range r.List {
		if pv.staged {
			kept = append(kept, pv)
		}
	}
	r.List = kept
	r.buildIndex()
}

// Find a PV
func (r *PersistentVolumes) FindPv(pv PersistentVolume) *PersistentVolume {
	r.init()
	i, found := r.index[pv.Name]
	if found {
		return &r.List[i]
	}

	return nil
}

// Add (or update) Pv to the collection.
func (r *PersistentVolumes) AddPv(pv PersistentVolume) {
	r.init()
	pv.staged = true
	foundPv := r.FindPv(pv)
	if foundPv == nil {
		r.List = append(r.List, pv)
		r.index[pv.Name] = len(r.List) - 1
	} else {
		foundPv.Update(pv)
	}
}

// Delete a PV from the collection.
func (r *PersistentVolumes) DeletePv(names ...string) {
	r.init()
	for _, name := range names {
		i, found := r.index[name]
		if found {
			r.List = append(r.List[:i], r.List[i+1:]...)
			r.buildIndex()
		}
	}
}
