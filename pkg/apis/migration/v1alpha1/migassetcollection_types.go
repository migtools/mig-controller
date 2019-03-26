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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigAssetCollectionSpec defines the desired state of MigAssetCollection
type MigAssetCollectionSpec struct {
	Namespaces []string `json:"namespaces,omitempty"`
}

// MigAssetCollectionStatus defines the observed state of MigAssetCollection
type MigAssetCollectionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigAssetCollection is the Schema for the migassetcollections API
// +k8s:openapi-gen=true
type MigAssetCollection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigAssetCollectionSpec   `json:"spec,omitempty"`
	Status MigAssetCollectionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigAssetCollectionList contains a list of MigAssetCollection
type MigAssetCollectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigAssetCollection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigAssetCollection{}, &MigAssetCollectionList{})
}
