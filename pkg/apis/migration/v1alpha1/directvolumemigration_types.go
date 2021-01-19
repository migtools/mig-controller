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
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PVCToMigrate struct {
	*kapi.ObjectReference `json:",inline"`
	TargetStorageClass    string                            `json:"targetStorageClass"`
	TargetAccessModes     []kapi.PersistentVolumeAccessMode `json:"targetAccessModes,omitEmpty"`
	Verify                bool                              `json:"verify,omitEmpty"`
}

// DirectVolumeMigrationSpec defines the desired state of DirectVolumeMigration
type DirectVolumeMigrationSpec struct {
	SrcMigClusterRef            *kapi.ObjectReference `json:"srcMigClusterRef,omitempty"`
	DestMigClusterRef           *kapi.ObjectReference `json:"destMigClusterRef,omitempty"`
	PersistentVolumeClaims      []PVCToMigrate        `json:"persistentVolumeClaims,omitempty"`
	CreateDestinationNamespaces bool                  `json:"createDestinationNamespaces,omitempty"`
	DeleteProgressReportingCRs  bool                  `json:"deleteProgressReportingCRs,omitempty"`
}

// DirectVolumeMigrationStatus defines the observed state of DirectVolumeMigration
type DirectVolumeMigrationStatus struct {
	Conditions       `json:","`
	ObservedDigest   string         `json:"observedDigest"`
	StartTimestamp   *metav1.Time   `json:"startTimestamp,omitempty"`
	PhaseDescription string         `json:"phaseDescription"`
	Phase            string         `json:"phase,omitempty"`
	Itinerary        string         `json:"itinerary,omitempty"`
	Errors           []string       `json:"errors,omitempty"`
	SuccessfulPods   []*PodProgress `json:"successfulPods,omitempty"`
	FailedPods       []*PodProgress `json:"failedPods,omitempty"`
	RunningPods      []*PodProgress `json:"runningPods,omitempty"`
}

// TODO: Explore how to reliably get stunnel+rsync logs/status reported back to
// DirectVolumeMigrationStatus

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectVolumeMigration is the Schema for the direct pv migration API
// +kubebuilder:resource:path=directvolumemigrations,shortName=dvm
// +k8s:openapi-gen=true
type DirectVolumeMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DirectVolumeMigrationSpec   `json:"spec,omitempty"`
	Status DirectVolumeMigrationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectVolumeMigrationList contains a list of DirectVolumeMigration
type DirectVolumeMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DirectVolumeMigration `json:"items"`
}

type PodProgress struct {
	*kapi.ObjectReference       `json:",inline"`
	LastObservedProgressPercent string `json:"lastObservedProgressPercent,omitempty"`
	LastObservedTransferRate    string `json:"lastObservedTransferRate,omitempty"`
}

func (r *DirectVolumeMigration) GetSourceCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.SrcMigClusterRef)
}

func (r *DirectVolumeMigration) GetDestinationCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.DestMigClusterRef)
}

func (r *DirectVolumeMigration) GetMigrationForDVM(client k8sclient.Client) (*MigMigration, error) {
	return GetMigrationForDVM(client, r.OwnerReferences)
}

// Add (de-duplicated) errors.
func (r *DirectVolumeMigration) AddErrors(errors []string) {
	m := map[string]bool{}
	for _, e := range r.Status.Errors {
		m[e] = true
	}
	for _, error := range errors {
		_, found := m[error]
		if !found {
			r.Status.Errors = append(r.Status.Errors, error)
		}
	}
}

// HasErrors will notify about error presence on the DirectVolumeMigration resource
func (r *DirectVolumeMigration) HasErrors() bool {
	return len(r.Status.Errors) > 0
}

func init() {
	SchemeBuilder.Register(&DirectVolumeMigration{}, &DirectVolumeMigrationList{})
}
