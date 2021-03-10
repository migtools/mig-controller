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

// DirectVolumeMigrationProgressSpec defines the desired state of DirectVolumeMigrationProgress
type DirectVolumeMigrationProgressSpec struct {
	ClusterRef   *kapi.ObjectReference `json:"clusterRef,omitempty"`
	PodRef       *kapi.ObjectReference `json:"podRef,omitempty"`
	PodSelector  map[string]string     `json:"podSelector,omitempty"`
	BackOffLimit int                   `json:"backOffLimit,omitempty"`
}

const (
	// DVMPDoneLabelKey used to track progress of dvmp operations
	DVMPDoneLabelKey = "openshift.migration.io/dvmp-done"
)

// DirectVolumeMigrationProgressStatus defines the observed state of DirectVolumeMigrationProgress
type DirectVolumeMigrationProgressStatus struct {
	Conditions `json:",omitempty"`
	// RsyncPodStatus observed state of most recent Rsync attempt
	RsyncPodStatus `json:",inline"`
	// RsyncPodStatuses history of all Rsync attempts
	RsyncPodStatuses []RsyncPodStatus `json:"rsyncPodStatuses,omitempty"`
	// Succeded whether Rsync operation succeded
	Succeded bool `json:"succeded,omitempty"`
	// Failed whether Rsync operation failed
	Failed bool `json:"failed,omitempty"`
	// Completed wether Rsync operation completed
	Completed bool `json:"completed,omitempty"`
	// RsyncElapsedTime total elapsed time of Rsync operation
	RsyncElapsedTime *metav1.Duration `json:"rsyncElapsedTime,omitempty"`
	// TotalProgressPercentage cumulative percentage of all Rsync attempts
	TotalProgressPercentage string `json:"totalProgressPercentage,omitempty"`
	ObservedDigest          string `json:"observedDigest,omitempty"`
}

// RsyncPodStatus defines observed state of an Rsync attempt
type RsyncPodStatus struct {
	// PodName name of the Rsync Pod
	PodName string `json:"podName,omitempty"`
	// PodPhase phase of the Rsync Pod
	PodPhase kapi.PodPhase `json:"phase,omitempty"`
	// ExitCode exit code of terminated Rsync Pod
	ExitCode *int32 `json:"exitCode,omitempty"`
	// ContainerElapsedTime total execution time of Rsync Pod
	ContainerElapsedTime *metav1.Duration `json:"containerElapsedTime,omitempty"`
	// LogMessage few lines of tailed log of the Rsync Pod
	LogMessage string `json:"logMessage,omitempty"`
	// LastObservedProgressPercent progress of Rsync in percentage
	LastObservedProgressPercent string `json:"lastObservedProgressPercent,omitempty"`
	// LastObservedTransferRate rate of transfer of Rsync
	LastObservedTransferRate string `json:"lastObservedTransferRate,omitempty"`
	// CreationTimestamp pod creation time
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`
}

// RsyncPodExistsInHistory checks whether Rsync pod status is already part of the history
func (ds *DirectVolumeMigrationProgressStatus) RsyncPodExistsInHistory(podName string) bool {
	for _, podStatus := range ds.RsyncPodStatuses {
		if podStatus.PodName == podName {
			return true
		}
	}
	return false
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectVolumeMigrationProgress is the Schema for the directvolumemigrationprogresses API
// +kubebuilder:resource:path=directvolumemigrationprogresses,shortName=dvmp
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="Pod Name",type=string,JSONPath=".spec.podRef.name"
// +kubebuilder:printcolumn:name="Pod Namespace",type=string,JSONPath=".spec.podRef.namespace"
// +kubebuilder:printcolumn:name="Progress Percent",type=string,JSONPath=".status.lastObservedProgressPercent"
// +kubebuilder:printcolumn:name="Transfer Rate",type=string,JSONPath=".status.lastObservedTransferRate"
// +kubebuilder:printcolumn:name="age",type=date,JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
type DirectVolumeMigrationProgress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DirectVolumeMigrationProgressSpec   `json:"spec,omitempty"`
	Status DirectVolumeMigrationProgressStatus `json:"status,omitempty"`
}

func (d *DirectVolumeMigrationProgress) MarkReconciled() {
	u, _ := uuid.NewUUID()
	if d.Annotations == nil {
		d.Annotations = map[string]string{}
	}
	d.Annotations[TouchAnnotation] = u.String()
	d.Status.ObservedDigest = digest(d.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectVolumeMigrationProgressList contains a list of DirectVolumeMigrationProgress
type DirectVolumeMigrationProgressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DirectVolumeMigrationProgress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DirectVolumeMigrationProgress{}, &DirectVolumeMigrationProgressList{})
}
