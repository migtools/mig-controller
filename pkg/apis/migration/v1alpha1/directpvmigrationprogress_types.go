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
	"github.com/google/uuid"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DirectPVMigrationProgressSpec defines the desired state of DirectPVMigrationProgress
type DirectPVMigrationProgressSpec struct {
	ClusterRef *kapi.ObjectReference `json:"clusterRef,omitempty"`
	PodRef     *kapi.ObjectReference `json:"podRef,omitempty"`
}

// DirectPVMigrationProgressStatus defines the observed state of DirectPVMigrationProgress
type DirectPVMigrationProgressStatus struct {
	Conditions
	PodPhase       kapi.PodPhase `json:"phase,omitempty"`
	ObservedDigest string        `json:"observedDigest,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectPVMigrationProgress is the Schema for the directpvmigrationprogresses API
// +kubebuilder:resource:path=directpvmigrationprogresses,shortName=dvmp
// +k8s:openapi-gen=true
type DirectPVMigrationProgress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DirectPVMigrationProgressSpec   `json:"spec,omitempty"`
	Status DirectPVMigrationProgressStatus `json:"status,omitempty"`
}

func (d *DirectPVMigrationProgress) MarkReconciled() {
	u, _ := uuid.NewUUID()
	if d.Annotations == nil {
		d.Annotations = map[string]string{}
	}
	d.Annotations[TouchAnnotation] = u.String()
	d.Status.ObservedDigest = digest(d.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectPVMigrationProgressList contains a list of DirectPVMigrationProgress
type DirectPVMigrationProgressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DirectPVMigrationProgress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DirectPVMigrationProgress{}, &DirectPVMigrationProgressList{})
}
