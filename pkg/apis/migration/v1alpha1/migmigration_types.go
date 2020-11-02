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

// Cache Indexes.
const (
	PlanIndexField = "planRef"
)

// MigMigrationSpec defines the desired state of MigMigration
type MigMigrationSpec struct {
	MigPlanRef      *kapi.ObjectReference `json:"migPlanRef,omitempty"`
	Stage           bool                  `json:"stage"`
	QuiescePods     bool                  `json:"quiescePods,omitempty"`
	KeepAnnotations bool                  `json:"keepAnnotations,omitempty"`
	Verify          bool                  `json:"verify,omitempty"`
	Canceled        bool                  `json:"canceled,omitempty"`
	Rollback        bool                  `json:"rollback,omitempty"`
}

// MigMigrationStatus defines the observed state of MigMigration
type MigMigrationStatus struct {
	Conditions
	UnhealthyResources
	ObservedDigest string       `json:"observedDigest,omitempty"`
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`
	Phase          string       `json:"phase,omitempty"`
	Pipeline       []*Step      `json:"pipeline,omitempty"`
	Itinerary      string       `json:"itinerary,omitempty"`
	Errors         []string     `json:"errors,omitempty"`
}

// FindStep find step by name
func (s *MigMigrationStatus) FindStep(stepName string) *Step {
	for _, step := range s.Pipeline {
		if step.Name == stepName {
			return step
		}
	}
	return nil
}

// AddStep adds a new step to the pipeline
func (s *MigMigrationStatus) AddStep(step *Step) {
	found := s.FindStep(step.Name)
	if found == nil {
		s.Pipeline = append(
			s.Pipeline,
			step,
		)
	}
}

// ReflectPipeline reflects pipeline
func (s *MigMigrationStatus) ReflectPipeline() {
	for _, step := range s.Pipeline {
		if step.MarkedCompleted() {
			step.Phase = ""
			step.Message = "Completed"
			step.Progress = []string{}
		}
		if step.Failed {
			step.Message = "Failed"
		}
	}
}

// Step defines a task in a step of migration
type Step struct {
	Timed

	Name     string   `json:"name"`
	Phase    string   `json:"phase,omitempty"`
	Message  string   `json:"message,omitempty"`
	Progress []string `json:"progress,omitempty"`
	Failed   bool     `json:"failed,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigMigration is the Schema for the migmigrations API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=".spec.migPlanRef.name"
// +kubebuilder:printcolumn:name="Stage",type=string,JSONPath=".spec.stage"
// +kubebuilder:printcolumn:name="Rollback",type=string,JSONPath=".spec.rollback"
// +kubebuilder:printcolumn:name="Itinerary",type=string,JSONPath=".status.itinerary"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MigMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigMigrationSpec   `json:"spec,omitempty"`
	Status MigMigrationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigMigrationList contains a list of MigMigration
type MigMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigMigration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigMigration{}, &MigMigrationList{})
}

// GetPlan - Get the migration plan.
// Returns `nil` when the reference cannot be resolved.
func (r *MigMigration) GetPlan(client k8sclient.Client) (*MigPlan, error) {
	return GetPlan(client, r.Spec.MigPlanRef)
}

// Add (de-duplicated) errors.
func (r *MigMigration) AddErrors(errors []string) {
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

// HasErrors will notify about error presence on the MigMigration resource
func (r *MigMigration) HasErrors() bool {
	return len(r.Status.Errors) > 0
}
