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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	HookPhaseLabel       = "phase"
	HookOwnerLabel       = "owner"
	PreBackupHookPhase   = "PreBackup"
	PostBackupHookPhase  = "PostBackup"
	PreRestoreHookPhase  = "PreRestore"
	PostRestoreHookPhase = "PostRestore"
)

// MigHookSpec defines the desired state of MigHook
type MigHookSpec struct {
	// Implies whether the hook is a custom Ansible playbook or a pre-built image. The value of this field is required. This is a required field.
	Custom                bool   `json:"custom"`

	// Specifies the image of the hook to be executed. This is a required field.
	Image                 string `json:"image"`

	// Specifies the contents of the custom Ansible playbook in base64 format, it is used in conjunction with the custom boolean flag.
	Playbook              string `json:"playbook,omitempty"`

	// Specifies the cluster on which the hook is to be executed. This is a required field.
	TargetCluster         string `json:"targetCluster"`

	// Specifies the highest amount of time for which the hook will run.
	ActiveDeadlineSeconds int64  `json:"activeDeadlineSeconds,omitempty"`
}

// MigHookStatus defines the observed state of MigHook
type MigHookStatus struct {
	Conditions         `json:","`
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigHook is the Schema for the mighooks API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="TargetCluster",type=string,JSONPath=".spec.targetCluster"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MigHook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigHookSpec   `json:"spec,omitempty"`
	Status MigHookStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigHookList contains a list of MigHook
type MigHookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigHook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigHook{}, &MigHookList{})
}

// Get an existing hook job.
func (r *MigHook) GetPhaseJob(client k8sclient.Client, phase string, owner string) (*batchv1.Job, error) {
	list := batchv1.JobList{}
	labels := r.GetCorrelationLabels()
	labels[HookPhaseLabel] = phase
	labels[HookOwnerLabel] = owner
	err := client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil
}

// Get an existing configMap job.
func (r *MigHook) GetPhaseConfigMap(client k8sclient.Client, phase string, owner string) (*corev1.ConfigMap, error) {
	list := corev1.ConfigMapList{}
	labels := r.GetCorrelationLabels()
	labels[HookPhaseLabel] = phase
	labels[HookOwnerLabel] = owner
	err := client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil
}
