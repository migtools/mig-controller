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
	"context"
	"fmt"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// GetMigAssetCollection gets a referenced MigAssetCollection from a MigPlan
func (m *MigPlan) GetMigAssetCollection(c client.Client) (*MigAssetCollection, error) {
	assetsRef := m.Spec.MigAssetCollectionRef
	assets := &MigAssetCollection{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: assetsRef.Name, Namespace: assetsRef.Namespace}, assets)
	if err != nil {
		clog.Info(fmt.Sprintf("[mPlan] Error getting MigAssetCollection [%s/%s]", assetsRef.Namespace, assetsRef.Name))
		return nil, err
	}
	return assets, nil
}

// GetSrcMigCluster gets a referenced MigAssetCollection from a MigPlan
func (m *MigPlan) GetSrcMigCluster(c client.Client) (*MigCluster, error) {
	migClusterRef := m.Spec.SrcMigClusterRef
	migCluster := &MigCluster{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: migClusterRef.Name, Namespace: migClusterRef.Namespace}, migCluster)
	if err != nil {
		clog.Info(fmt.Sprintf("[mPlan] Error getting SrcMigCluster [%s/%s]", migClusterRef.Namespace, migClusterRef.Name))
		return nil, err
	}
	return migCluster, nil
}

// GetDestMigCluster gets a referenced MigAssetCollection from a MigPlan
func (m *MigPlan) GetDestMigCluster(c client.Client) (*MigCluster, error) {
	migClusterRef := m.Spec.DestMigClusterRef
	migCluster := &MigCluster{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: migClusterRef.Name, Namespace: migClusterRef.Namespace}, migCluster)
	if err != nil {
		clog.Info(fmt.Sprintf("[mPlan] Error getting DestMigCluster [%s/%s]", migClusterRef.Namespace, migClusterRef.Name))
		return nil, err
	}
	return migCluster, nil
}
