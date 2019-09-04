package v1alpha1

import (
	"github.com/google/uuid"
)

const (
	TouchAnnotation = "touch"
	VeleroNamespace = "migration"
)

// Migration application CR.
// The `touch` UUID is set by the controller after reconcile
// and needed to support relay of remote watch events. Also to
// provide remote watch predicates the ability to determine
// when an updated referenced resource has been fully reconciled.
type MigResource interface {
	// Get a map containing the correlation label.
	GetCorrelationLabels() map[string]string
	// Get the correlation label (key, value).
	GetCorrelationLabel() (string, string)
	// Get the resource namespace.
	GetNamespace() string
	// Get the resource name.
	GetName() string
	// Get the `touch` UUID annotation.
	GetTouch() string
	// Set the `touch` UUID annotation.
	Touch()
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

func (r *MigPlan) GetTouch() string {
	if r.Annotations == nil {
		return ""
	}
	id, found := r.Annotations[TouchAnnotation]
	if found {
		return id
	}

	return ""
}

func (r *MigPlan) Touch() {
	if r.Annotations == nil {
		r.Annotations = make(map[string]string)
	}
	r.Annotations[TouchAnnotation] = uuid.New().String()
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

func (r *MigStorage) GetTouch() string {
	if r.Annotations == nil {
		return ""
	}
	id, found := r.Annotations[TouchAnnotation]
	if found {
		return id
	}

	return ""
}

func (r *MigStorage) Touch() {
	if r.Annotations == nil {
		r.Annotations = make(map[string]string)
	}
	r.Annotations[TouchAnnotation] = uuid.New().String()
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

func (r *MigCluster) GetTouch() string {
	if r.Annotations == nil {
		return ""
	}
	id, found := r.Annotations[TouchAnnotation]
	if found {
		return id
	}

	return ""
}

func (r *MigCluster) Touch() {
	if r.Annotations == nil {
		r.Annotations = make(map[string]string)
	}
	r.Annotations[TouchAnnotation] = uuid.New().String()
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

func (r *MigMigration) GetTouch() string {
	if r.Annotations == nil {
		return ""
	}
	id, found := r.Annotations[TouchAnnotation]
	if found {
		return id
	}

	return ""
}

func (r *MigMigration) Touch() {
	if r.Annotations == nil {
		r.Annotations = make(map[string]string)
	}
	r.Annotations[TouchAnnotation] = uuid.New().String()
}
