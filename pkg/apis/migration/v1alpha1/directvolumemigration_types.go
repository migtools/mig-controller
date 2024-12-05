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
	"fmt"
	"slices"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type EndpointType string

const (
	Route     EndpointType = "Route"
	ClusterIP EndpointType = "ClusterIP"
	NodePort  EndpointType = "NodePort"
)

const RSYNC_ENDPOINT_TYPE = "RSYNC_ENDPOINT_TYPE"

type PVCToMigrate struct {
	*kapi.ObjectReference `json:",inline"`
	// TargetStorageClass storage class of the migrated PVC in the target cluster
	TargetStorageClass string `json:"targetStorageClass"`
	// TargetAccessModes access modes of the migrated PVC in the target cluster
	TargetAccessModes []kapi.PersistentVolumeAccessMode `json:"targetAccessModes"`
	// TargetVolumeMode volume mode of the migrated PVC in the target cluster
	TargetVolumeMode *kapi.PersistentVolumeMode `json:"targetVolumeMode,omitempty"`
	// TargetNamespace namespace of the migrated PVC in the target cluster
	TargetNamespace string `json:"targetNamespace,omitempty"`
	// TargetName name of the migrated PVC in the target cluster
	// +kubebuilder:validation:Optional
	TargetName string `json:"targetName,omitempty"`
	// Verify set true to verify integrity of the data post migration
	Verify bool `json:"verify,omitempty"`
}

// DirectVolumeMigrationSpec defines the desired state of DirectVolumeMigration
type DirectVolumeMigrationSpec struct {
	SrcMigClusterRef  *kapi.ObjectReference `json:"srcMigClusterRef,omitempty"`
	DestMigClusterRef *kapi.ObjectReference `json:"destMigClusterRef,omitempty"`

	// BackOffLimit retry limit on Rsync pods
	BackOffLimit int `json:"backOffLimit,omitempty"`

	//  Holds all the PVCs that are to be migrated with direct volume migration
	PersistentVolumeClaims []PVCToMigrate `json:"persistentVolumeClaims,omitempty"`

	// Set true to create namespaces in destination cluster
	CreateDestinationNamespaces bool `json:"createDestinationNamespaces,omitempty"`

	// Specifies if progress reporting CRs needs to be deleted or not
	DeleteProgressReportingCRs bool `json:"deleteProgressReportingCRs,omitempty"`

	// Specifies if any volumes associated with a VM should be live storage migrated instead of offline migrated
	LiveMigrate *bool `json:"liveMigrate,omitempty"`

	// Specifies if this is the final DVM in the migration plan
	MigrationType *DirectVolumeMigrationType `json:"migrationType,omitempty"`
}

type DirectVolumeMigrationType string

const (
	MigrationTypeStage    DirectVolumeMigrationType = "Stage"
	MigrationTypeFinal    DirectVolumeMigrationType = "CutOver"
	MigrationTypeRollback DirectVolumeMigrationType = "Rollback"
)

// DirectVolumeMigrationStatus defines the observed state of DirectVolumeMigration
type DirectVolumeMigrationStatus struct {
	Conditions                `json:","`
	ObservedDigest            string                   `json:"observedDigest"`
	StartTimestamp            *metav1.Time             `json:"startTimestamp,omitempty"`
	PhaseDescription          string                   `json:"phaseDescription"`
	Phase                     string                   `json:"phase,omitempty"`
	Itinerary                 string                   `json:"itinerary,omitempty"`
	Errors                    []string                 `json:"errors,omitempty"`
	SuccessfulPods            []*PodProgress           `json:"successfulPods,omitempty"`
	FailedPods                []*PodProgress           `json:"failedPods,omitempty"`
	RunningPods               []*PodProgress           `json:"runningPods,omitempty"`
	PendingPods               []*PodProgress           `json:"pendingPods,omitempty"`
	UnknownPods               []*PodProgress           `json:"unknownPods,omitempty"`
	PendingSinceTimeLimitPods []*PodProgress           `json:"pendingSinceTimeLimitPods,omitempty"`
	SuccessfulLiveMigrations  []*LiveMigrationProgress `json:"successfulLiveMigration,omitempty"`
	RunningLiveMigrations     []*LiveMigrationProgress `json:"runningLiveMigration,omitempty"`
	PendingLiveMigrations     []*LiveMigrationProgress `json:"pendingLiveMigration,omitempty"`
	FailedLiveMigrations      []*LiveMigrationProgress `json:"failedLiveMigration,omitempty"`
	RsyncOperations           []*RsyncOperation        `json:"rsyncOperations,omitempty"`
	SkippedVolumes            []string                 `json:"skippedVolumes,omitempty"`
}

// GetRsyncOperationStatusForPVC returns RsyncOperation from status for matching PVC, creates new one if doesn't exist already
func (ds *DirectVolumeMigrationStatus) GetRsyncOperationStatusForPVC(pvcRef *kapi.ObjectReference) *RsyncOperation {
	for i := range ds.RsyncOperations {
		rsyncOperation := ds.RsyncOperations[i]
		if rsyncOperation.PVCReference.Namespace == pvcRef.Namespace &&
			rsyncOperation.PVCReference.Name == pvcRef.Name {
			return rsyncOperation
		}
	}
	newStatus := &RsyncOperation{
		PVCReference:   pvcRef,
		CurrentAttempt: 0,
	}
	ds.RsyncOperations = append(ds.RsyncOperations, newStatus)
	return newStatus
}

// AddRsyncOperation adds a new RsyncOperation to list, updates an existing one if found
func (ds *DirectVolumeMigrationStatus) AddRsyncOperation(podStatus *RsyncOperation) {
	if podStatus == nil {
		return
	}
	for i := range ds.RsyncOperations {
		existing := ds.RsyncOperations[i]
		if existing.Equal(podStatus) {
			existing.CurrentAttempt = podStatus.CurrentAttempt
			existing.Failed = podStatus.Failed
			existing.Succeeded = podStatus.Succeeded
			return
		}
	}
	ds.RsyncOperations = append(ds.RsyncOperations, podStatus)
}

func (dvm *DirectVolumeMigration) IsCompleted() bool {
	return len(dvm.Status.SuccessfulPods)+
		len(dvm.Status.FailedPods)+
		len(dvm.Status.SkippedVolumes)+
		len(dvm.Status.SuccessfulLiveMigrations)+
		len(dvm.Status.FailedLiveMigrations) == len(dvm.Spec.PersistentVolumeClaims)
}

func (dvm *DirectVolumeMigration) IsCutover() bool {
	return dvm.Spec.MigrationType != nil && *dvm.Spec.MigrationType == MigrationTypeFinal
}

func (dvm *DirectVolumeMigration) IsRollback() bool {
	return dvm.Spec.MigrationType != nil && *dvm.Spec.MigrationType == MigrationTypeRollback
}

func (dvm *DirectVolumeMigration) IsStage() bool {
	return dvm.Spec.MigrationType != nil && *dvm.Spec.MigrationType == MigrationTypeStage
}

func (dvm *DirectVolumeMigration) SkipVolume(volumeName, namespace string) {
	dvm.Status.SkippedVolumes = append(dvm.Status.SkippedVolumes, fmt.Sprintf("%s/%s", namespace, volumeName))
}

func (dvm *DirectVolumeMigration) IsLiveMigrate() bool {
	return dvm.Spec.LiveMigrate != nil && *dvm.Spec.LiveMigrate
}

func (dvm *DirectVolumeMigration) AllReportingCompleted() bool {
	isCompleted := dvm.IsCompleted()
	isAnyPending := len(dvm.Status.PendingPods) > 0 || len(dvm.Status.PendingLiveMigrations) > 0
	isAnyRunning := len(dvm.Status.RunningPods) > 0 || len(dvm.Status.RunningLiveMigrations) > 0
	isAnyUnknown := len(dvm.Status.UnknownPods) > 0
	return !isAnyRunning && !isAnyPending && !isAnyUnknown && isCompleted
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
	PVCReference                *kapi.ObjectReference `json:"pvcRef,omitempty"`
	LastObservedProgressPercent string                `json:"lastObservedProgressPercent,omitempty"`
	LastObservedTransferRate    string                `json:"lastObservedTransferRate,omitempty"`
	TotalElapsedTime            *metav1.Duration      `json:"totalElapsedTime,omitempty"`
}

type LiveMigrationProgress struct {
	VMName                      string                `json:"vmName,omitempty"`
	VMNamespace                 string                `json:"vmNamespace,omitempty"`
	PVCReference                *kapi.ObjectReference `json:"pvcRef,omitempty"`
	LastObservedProgressPercent string                `json:"lastObservedProgressPercent,omitempty"`
	LastObservedTransferRate    string                `json:"lastObservedTransferRate,omitempty"`
	TotalElapsedTime            *metav1.Duration      `json:"totalElapsedTime,omitempty"`
	Message                     string                `json:"message,omitempty"`
}

// RsyncOperation defines observed state of an Rsync Operation
type RsyncOperation struct {
	// PVCReference pvc to which this Rsync operation corresponds to
	PVCReference *kapi.ObjectReference `json:"pvcReference,omitempty"`
	// CurrentAttempt current ongoing attempt of an Rsync operation
	CurrentAttempt int `json:"currentAttempt,omitempty"`
	// Succeeded whether operation as a whole succeded
	Succeeded bool `json:"succeeded,omitempty"`
	// Failed whether operation as a whole failed
	Failed bool `json:"failed,omitempty"`
}

func (x *RsyncOperation) Equal(y *RsyncOperation) bool {
	if y == nil || x.PVCReference == nil || y.PVCReference == nil {
		return false
	}
	if x.PVCReference.Name == y.PVCReference.Name &&
		x.PVCReference.Namespace == y.PVCReference.Namespace {
		return true
	}
	return false
}

func (r *RsyncOperation) GetPVDetails() (string, string) {
	if r.PVCReference != nil {
		return r.PVCReference.Namespace, r.PVCReference.Name
	}
	return "", ""
}

func (r *RsyncOperation) String() string {
	if r.PVCReference != nil {
		return fmt.Sprintf("%s/%s", r.PVCReference.Namespace, r.PVCReference.Name)
	}
	return ""
}

// IsComplete tells whether the operation as a whole is in terminal state
func (r *RsyncOperation) IsComplete() bool {
	return r.Failed || r.Succeeded
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

func (r *DirectVolumeMigration) GetSourceNamespaces() []string {
	namespaces := []string{}
	for _, pvc := range r.Spec.PersistentVolumeClaims {
		if pvc.Namespace != "" && !slices.Contains(namespaces, pvc.Namespace) {
			namespaces = append(namespaces, pvc.Namespace)
		}
	}
	return namespaces
}

func (r *DirectVolumeMigration) GetDestinationNamespaces() []string {
	namespaces := []string{}
	for _, pvc := range r.Spec.PersistentVolumeClaims {
		if pvc.TargetNamespace != "" && !slices.Contains(namespaces, pvc.TargetNamespace) {
			namespaces = append(namespaces, pvc.TargetNamespace)
		}
	}
	return namespaces
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
