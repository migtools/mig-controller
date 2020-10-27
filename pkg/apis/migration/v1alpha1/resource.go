package v1alpha1

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
)

const (
	TouchAnnotation = "openshift.io/touch"
	VeleroNamespace = "openshift-migration"
)

// Migration application CR.
type MigResource interface {
	// Get a map containing the correlation label.
	// Correlation labels are used to track any resource
	// created by the controller.  The includes the application
	// (global) label and the resource label.
	GetCorrelationLabels() map[string]string
	// Get the resource correlation label.
	// The label is used to track resources created by the
	// controller that is related to this resource.
	GetCorrelationLabel() (string, string)
	// Get the resource namespace.
	GetNamespace() string
	// Get the resource name.
	GetName() string
	// Mark the resource as having been reconciled.
	// Updates the ObservedDigest.
	// Update the touch annotation. This ensures that the resource
	// is changed on every reconcile. This needed to support
	// remote watch event propagation on OCP4.
	MarkReconciled()
	// Get whether the resource has been reconciled.
	HasReconciled() bool
}

// Plan
func (r *MigPlan) GetCorrelationLabels() map[string]string {
	key, value := r.GetCorrelationLabel()
	return map[string]string{
		PartOfLabel: Application,
		key:         value,
	}
}

func (r *MigPlan) GetCorrelationLabel() (string, string) {
	return CorrelationLabel(r, r.UID)
}

func (r *MigPlan) GetNamespace() string {
	return r.Namespace
}

func (r *MigPlan) GetName() string {
	return r.Name
}

func (r *MigPlan) MarkReconciled() {
	uuid, _ := uuid.NewUUID()
	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	r.Annotations[TouchAnnotation] = uuid.String()
	r.Status.ObservedDigest = digest(r.Spec)
}

func (r *MigPlan) HasReconciled() bool {
	return r.Status.ObservedDigest == digest(r.Spec)
}

// Storage
func (r *MigStorage) GetCorrelationLabels() map[string]string {
	key, value := r.GetCorrelationLabel()
	return map[string]string{
		PartOfLabel: Application,
		key:         value,
	}
}

func (r *MigStorage) GetCorrelationLabel() (string, string) {
	return CorrelationLabel(r, r.UID)
}

func (r *MigStorage) GetNamespace() string {
	return r.Namespace
}

func (r *MigStorage) GetName() string {
	return r.Name
}

func (r *MigStorage) MarkReconciled() {
	uuid, _ := uuid.NewUUID()
	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	r.Annotations[TouchAnnotation] = uuid.String()
	r.Status.ObservedDigest = digest(r.Spec)
}

func (r *MigStorage) HasReconciled() bool {
	return r.Status.ObservedDigest == digest(r.Spec)
}

//Analytic
func (r *MigAnalytic) GetCorrelationLabels() map[string]string {
	key, value := r.GetCorrelationLabel()
	return map[string]string{
		PartOfLabel: Application,
		key:         value,
	}
}

func (r *MigAnalytic) GetCorrelationLabel() (string, string) {
	return CorrelationLabel(r, r.UID)
}

func (r *MigAnalytic) GetNamespace() string {
	return r.Namespace
}

func (r *MigAnalytic) GetName() string {
	return r.Name
}

func (r *MigAnalytic) MarkReconciled() {
	r.Status.ObservedGeneration = r.Generation + 1
}

func (r *MigAnalytic) HasReconciled() bool {
	return r.Status.ObservedGeneration == r.Generation
}

//Hook
func (r *MigHook) GetCorrelationLabels() map[string]string {
	key, value := r.GetCorrelationLabel()
	return map[string]string{
		PartOfLabel: Application,
		key:         value,
	}
}

func (r *MigHook) GetCorrelationLabel() (string, string) {
	return CorrelationLabel(r, r.UID)
}

func (r *MigHook) GetNamespace() string {
	return r.Namespace
}

func (r *MigHook) GetName() string {
	return r.Name
}

func (r *MigHook) MarkReconciled() {
	r.Status.ObservedGeneration = r.Generation + 1
}

func (r *MigHook) HasReconciled() bool {
	return r.Status.ObservedGeneration == r.Generation
}

// Direct
func (r *DirectVolumeMigration) GetCorrelationLabels() map[string]string {
	key, value := r.GetCorrelationLabel()
	return map[string]string{
		PartOfLabel: Application,
		key:         value,
	}
}

func (r *DirectVolumeMigration) GetCorrelationLabel() (string, string) {
	return CorrelationLabel(r, r.UID)
}

func (r *DirectVolumeMigration) GetNamespace() string {
	return r.Namespace
}

func (r *DirectVolumeMigration) GetName() string {
	return r.Name
}

func (r *DirectVolumeMigration) MarkReconciled() {
	uuid, _ := uuid.NewUUID()
	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	r.Annotations[TouchAnnotation] = uuid.String()
	r.Status.ObservedDigest = digest(r.Spec)
}

func (r *DirectVolumeMigration) HasReconciled() bool {
	return r.Status.ObservedDigest == digest(r.Spec)
}

// Cluster
func (r *MigCluster) GetCorrelationLabels() map[string]string {
	key, value := r.GetCorrelationLabel()
	return map[string]string{
		PartOfLabel: Application,
		key:         value,
	}
}

func (r *MigCluster) GetCorrelationLabel() (string, string) {
	return CorrelationLabel(r, r.UID)
}

func (r *MigCluster) GetNamespace() string {
	return r.Namespace
}

func (r *MigCluster) GetName() string {
	return r.Name
}

func (r *MigCluster) MarkReconciled() {
	uuid, _ := uuid.NewUUID()
	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	r.Annotations[TouchAnnotation] = uuid.String()
	r.Status.ObservedDigest = digest(r.Spec)
}

func (r *MigCluster) HasReconciled() bool {
	return r.Status.ObservedDigest == digest(r.Spec)
}

// Migration
func (r *MigMigration) GetCorrelationLabels() map[string]string {
	key, value := r.GetCorrelationLabel()
	return map[string]string{
		PartOfLabel: Application,
		key:         value,
	}
}

func (r *MigMigration) GetCorrelationLabel() (string, string) {
	return CorrelationLabel(r, r.UID)
}

func (r *MigMigration) GetNamespace() string {
	return r.Namespace
}

func (r *MigMigration) GetName() string {
	return r.Name
}

func (r *MigMigration) MarkReconciled() {
	uuid, _ := uuid.NewUUID()
	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	r.Annotations[TouchAnnotation] = uuid.String()
	r.Status.ObservedDigest = digest(r.Spec)
}

func (r *MigMigration) HasReconciled() bool {
	return r.Status.ObservedDigest == digest(r.Spec)
}

//
// Generate a sha256 hex-digest for an object.
func digest(object interface{}) string {
	j, _ := json.Marshal(object)
	hash := sha256.New()
	hash.Write(j)
	digest := hex.EncodeToString(hash.Sum(nil))
	return digest
}
