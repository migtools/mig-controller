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
	"time"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MigStageSpec defines the desired state of MigStage
type MigStageSpec struct {
	MigPlanRef *kapi.ObjectReference `json:"migPlanRef,omitempty"`
}

// MigStageStatus defines the observed state of MigStage
type MigStageStatus struct {
	Conditions
	StageRunning        bool         `json:"stageStarted,omitempty"`
	StageCompleted      bool         `json:"stageCompleted,omitempty"`
	StartTimestamp      *metav1.Time `json:"startTimestamp,omitempty"`
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`
	TaskPhase           string       `json:"taskPhase,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigStage is the Schema for the migstages API
// +k8s:openapi-gen=true
type MigStage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigStageSpec   `json:"spec,omitempty"`
	Status MigStageStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigStageList contains a list of MigStage
type MigStageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigStage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigStage{}, &MigStageList{})
}

// GetPlan - Get the migration plan.
// Returns `nil` when the reference cannot be resolved.
func (r *MigStage) GetPlan(client k8sclient.Client) (*MigPlan, error) {
	return GetPlan(client, r.Spec.MigPlanRef)
}

// MarkAsRunning marks the MigStage status as 'Running'. Returns true if changed.
func (r *MigStage) MarkAsRunning() bool {
	if r.Status.StageCompleted == true || r.Status.StageRunning == true {
		return false
	}
	r.Status.StageRunning = true
	r.Status.StageCompleted = false
	r.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	return true
}

// MarkAsCompleted marks the MigStage status as 'Completed'. Returns true if changed.
func (r *MigStage) MarkAsCompleted() bool {
	if r.Status.StageRunning == false || r.Status.StageCompleted == true {
		return false
	}
	r.Status.StageRunning = false
	r.Status.StageCompleted = true
	r.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
	return true
}

// Get whether the stage has completed
func (r *MigStage) IsCompleted() bool {
	return r.Status.StageCompleted
}
