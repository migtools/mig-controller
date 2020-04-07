package v1alpha1

const (
	VeleroNamespace = "openshift-migration"
	ProxySecret     = "migration-proxy"
)

// Migration application CR.
type MigResource interface {
	// Get a map containing the correlation label.
	GetCorrelationLabels() map[string]string
	// Get the correlation label (key, value).
	GetCorrelationLabel() (string, string)
	// Get the resource namespace.
	GetNamespace() string
	// Get the resource name.
	GetName() string
	// Mark the resource as having been reconciled.
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
	r.Status.ObservedGeneration = r.Generation + 1
}

func (r *MigPlan) HasReconciled() bool {
	return r.Status.ObservedGeneration == r.Generation
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
	r.Status.ObservedGeneration = r.Generation + 1
}

func (r *MigStorage) HasReconciled() bool {
	return r.Status.ObservedGeneration == r.Generation
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
	r.Status.ObservedGeneration = r.Generation + 1
}

func (r *MigCluster) HasReconciled() bool {
	return r.Status.ObservedGeneration == r.Generation
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
	r.Status.ObservedGeneration = r.Generation + 1
}

func (r *MigMigration) HasReconciled() bool {
	return r.Status.ObservedGeneration == r.Generation
}
