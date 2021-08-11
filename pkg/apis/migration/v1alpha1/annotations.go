package v1alpha1

// Velero Plugin Annotations
const (
	StageOrFinalMigrationAnnotation = "migration.openshift.io/migmigration-type" // (stage|final)
	StageMigration                  = "stage"
	FinalMigration                  = "final"
	PvActionAnnotation              = "openshift.io/migrate-type"          // (move|copy)
	PvStorageClassAnnotation        = "openshift.io/target-storage-class"  // storageClassName
	PvAccessModeAnnotation          = "openshift.io/target-access-mode"    // accessMode
	PvCopyMethodAnnotation          = "migration.openshift.io/copy-method" // (snapshot|filesystem)
	QuiesceAnnotation               = "openshift.io/migrate-quiesce-pods"  // (true|false)
	QuiesceNodeSelector             = "migration.openshift.io/quiesceDaemonSet"
	SuspendAnnotation               = "migration.openshift.io/preQuiesceSuspend"
	ReplicasAnnotation              = "migration.openshift.io/preQuiesceReplicas"
	NodeSelectorAnnotation          = "migration.openshift.io/preQuiesceNodeSelector"
	StagePodImageAnnotation         = "migration.openshift.io/stage-pod-image"
)

// Restic Annotations
const (
	ResticPvBackupAnnotation = "backup.velero.io/backup-volumes" // comma-separated list of volume names
	ResticPvVerifyAnnotation = "backup.velero.io/verify-volumes" // comma-separated list of volume names
)

// Migration Annotations
const (
	// Disables the internal image copy
	DisableImageCopy         = "migration.openshift.io/disable-image-copy"
	StateMigrationAnnotation = "migration.openshift.io/state-transfer"
)
