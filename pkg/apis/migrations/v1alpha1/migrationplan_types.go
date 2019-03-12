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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MigrationPlanSpec defines the desired state of MigrationPlan
type MigrationPlanSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SrcClusterRef               *kapi.ObjectReference `json:"srcClusterRef,omitempty"`
	DestClusterRef              *kapi.ObjectReference `json:"destClusterRef,omitempty"`
	MigrationStorageRef         *kapi.ObjectReference `json:"migrationStorageRef,omitempty"`
	MigrationAssetCollectionRef *kapi.ObjectReference `json:"migrationAssetCollectionRef,omitempty"`
}

// MigrationPlanStatus defines the observed state of MigrationPlan
type MigrationPlanStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigrationPlan is the Schema for the migrationplans API
// +k8s:openapi-gen=true
type MigrationPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationPlanSpec   `json:"spec,omitempty"`
	Status MigrationPlanStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigrationPlanList contains a list of MigrationPlan
type MigrationPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigrationPlan{}, &MigrationPlanList{})
}
