package v1alpha1

// Migration application CR.
type MigResource interface {
	GetCorrelationLabels() map[string]string
	GetNamespace() string
	GetName() string
}

// Plan
func (r *MigPlan) GetCorrelationLabels() map[string]string {
	return buildCorrelationLabels(r, r.UID)
}

func (r *MigPlan) GetNamespace() string {
	return r.Namespace
}

func (r *MigPlan) GetName() string {
	return r.Name
}

// Storage
func (r *MigStorage) GetCorrelationLabels() map[string]string {
	return buildCorrelationLabels(r, r.UID)
}

func (r *MigStorage) GetNamespace() string {
	return r.Namespace
}

func (r *MigStorage) GetName() string {
	return r.Name
}

// Cluster
func (r *MigCluster) GetCorrelationLabels() map[string]string {
	return buildCorrelationLabels(r, r.UID)
}

func (r *MigCluster) GetNamespace() string {
	return r.Namespace
}

func (r *MigCluster) GetName() string {
	return r.Name
}

// Migration
func (r *MigMigration) GetCorrelationLabels() map[string]string {
	return buildCorrelationLabels(r, r.UID)
}

func (r *MigMigration) GetNamespace() string {
	return r.Namespace
}

func (r *MigMigration) GetName() string {
	return r.Name
}
