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
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigPlanSpec defines the desired state of MigPlan
type MigPlanSpec struct {
	SrcClusterRef         *kapi.ObjectReference `json:"srcClusterRef,omitempty"`
	DestClusterRef        *kapi.ObjectReference `json:"destClusterRef,omitempty"`
	MigStorageRef         *kapi.ObjectReference `json:"migStorageRef,omitempty"`
	MigAssetCollectionRef *kapi.ObjectReference `json:"migAssetCollectionRef,omitempty"`
}

// MigPlanStatus defines the observed state of MigPlan
type MigPlanStatus struct {
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
