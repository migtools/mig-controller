package v1alpha1

import (
	"strings"

	migref "github.com/konveyor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/types"
)

// Common labels
const (
	PartOfLabel = "app.kubernetes.io/part-of" // = Application
	Application = "openshift-migration"
)

// Build label (key, value) used to correlate CRs.
// Format: <kind>: <uid>.  The <uid> should be the ObjectMeta.UID
func CorrelationLabel(r interface{}, uid types.UID) (key, value string) {
	return labelKey(r), string(uid)
}

// Get a label (key) for the specified CR kind.
func labelKey(r interface{}) string {
	return strings.ToLower(migref.ToKind(r))
}

// Labels.
const (
	// Resources included in the stage backup.
	// Referenced by the Backup.LabelSelector. The value is the Task.UID().
	IncludedInStageBackupLabel = "migration-included-stage-backup"
	// Designated as an `initial` Backup.
	// The value is the Task.UID().
	InitialBackupLabel = "migration-initial-backup"
	// Designated as an `stage` Backup.
	// The value is the Task.UID().
	StageBackupLabel = "migration-stage-backup"
	// Designated as an `stage` Restore.
	// The value is the Task.UID().
	StageRestoreLabel = "migration-stage-restore"
	// Designated as a `final` Restore.
	// The value is the Task.UID().
	FinalRestoreLabel = "migration-final-restore"
	// Identifies associated directvolumemigration resource
	// The value is the Task.UID()
	DirectVolumeMigrationLabel = "migration-direct-volume"
	// Identifies the resource as migrated by us
	// for easy search or application rollback.
	// The value is the Task.UID().
	MigMigrationLabel = "migration.openshift.io/migrated-by-migmigration" // (migmigration UID)
	// Identifies associated migmigration
	// to assist manual debugging
	// The value is Task.Owner.Name
	MigMigrationDebugLabel = "migration.openshift.io/migmigration-name"
	// Identifies associated migplan
	// to assist manual debugging
	// The value is Task.Owner.Spec.migPlanRef.Name
	MigPlanDebugLabel = "migration.openshift.io/migplan-name"
	// Identifies associated migplan
	// to allow migplan restored resources rollback
	// The value is Task.PlanResources.MigPlan.UID
	MigPlanLabel = "migration.openshift.io/migrated-by-migplan" // (migplan UID)
	// Identifies associated Backup name
	MigBackupLabel = "migration.openshift.io/migrated-by-backup" // (backup name)
	// Identifies Pod as a stage pod to allow
	// for cleanup at migration start and rollback
	// The value is always "true" if set.
	StagePodLabel = "migration.openshift.io/is-stage-pod"
	// RsyncPodIdentityLabel identifies sibling Rsync attempts/pods
	RsyncPodIdentityLabel = "migration.openshift.io/created-for-pvc"
	// Identifies if the pod is application pod or not
	ApplicationPodLabel = "migration.openshift.io/is-application-pod"
)
