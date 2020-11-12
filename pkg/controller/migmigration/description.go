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

package migmigration

// PhaseDescriptions are human readable strings that describe a phase
var PhaseDescriptions = map[string]string{
	Started:                         "Migration started.",
	StartRefresh:                    "Starting refresh on MigPlan, MigStorage and MigCluster resources",
	WaitForRefresh:                  "Waiting for refresh of MigPlan, MigStorage and MigCluster resources to complete",
	CleanStaleAnnotations:           "Removing leftover migration annotations and labels from PVs, PVCs, Pods, ImageStreams, and Namespaces. Annotations and labels provide migration instructions to Velero, Velero Plugins and Restic.",
	CleanStaleResticCRs:             "Deleting incomplete Restic PodVolumeBackups and PodVolumeRestores created by past migration attempts.",
	CleanStaleVeleroCRs:             "Deleting incomplete Velero Backups and Restores created by past migration attempts.",
	CleanStaleStagePods:             "Deleting any leftover stage Pods.",
	WaitForStaleStagePodsTerminated: "Waiting for leftover stage Pod deletion to finish.",
	CreateRegistries:                "Creating migration registries on source and target clusters.",
	WaitForRegistriesReady:          "Waiting for migration registries on source and target clusters to become healthy.",
	DeleteRegistries:                "Deleting migration registries on source and target clusters.",
	EnsureCloudSecretPropagated:     "Ensuring Velero has latest Replication Repository storage credentials.",
	PreBackupHooks:                  "Waiting for user-defined pre-backup hooks to complete.",
	PostBackupHooks:                 "Waiting for user-defined post-backup hooks to complete.",
	PreRestoreHooks:                 "Waiting for user-defined pre-restore hooks to complete.",
	PostRestoreHooks:                "Waiting for user-defined post-restore hooks to complete.",
	PreBackupHooksFailed:            "Migration failed while running user-defined pre-backup hooks.",
	PostBackupHooksFailed:           "Migration failed while running user-defined post-backup hooks.",
	PreRestoreHooksFailed:           "Migration failed while running user-defined pre-restore hooks.",
	PostRestoreHooksFailed:          "Migration failed while running user-defined post-restore hooks.",
	EnsureInitialBackup:             "Creating initial Velero backup.",
	InitialBackupCreated:            "Waiting for initial Velero backup to complete.",
	InitialBackupFailed:             "Migration failed during initial Velero backup.",
	AnnotateResources:               "Adding migration annotations and labels to PVs, PVCs, Pods, ImageStreams, and Namespaces. Annotations and labels provide migration instructions to Velero, Velero Plugins and Restic.",
	EnsureStagePodsFromRunning:      "Creating Stage Pods and mounting PVC data from running Pods.",
	EnsureStagePodsFromTemplates:    "Creating Stage Pods and mounting PVC data using Pod templates from DeploymentTemplates, DeploymentConfigs, ReplicationControllers, DaemonSets, StatefulSets, ReplicaSets, CronJobs and Jobs.",
	EnsureStagePodsFromOrphanedPVCs: "Creating Stage Pods and mounting PVC data from unmounted PVCs.",
	StagePodsCreated:                "Waiting for all Stage Pods to start.",
	StagePodsFailed:                 "Migration failed due to some Stage Pods failing to start.",
	RestartRestic:                   "Restarting Restic Pods, ensuring latest PVC mounts are available for PVC backups.",
	WaitForResticReady:              "Waiting for Restic Pods to restart, ensuring latest PVC mounts are available for PVC backups.",
	RestartVelero:                   "Restarting Velero Pods, ensuring work queue is empty.",
	WaitForVeleroReady:              "Waiting for Velero Pods to restart, ensuring work queue is empty.",
	QuiesceApplications:             "Quiescing (Scaling to 0 replicas): Deployments, DeploymentConfigs, StatefulSets, ReplicaSets, DaemonSets, CronJobs and Jobs.",
	EnsureQuiesced:                  "Waiting for Quiesce (Scaling to 0 replicas) to finish for Deployments, DeploymentConfigs, StatefulSets, ReplicaSets, DaemonSets, CronJobs and Jobs.",
	UnQuiesceApplications:           "UnQuiescing (Scaling to N replicas) Deployments, DeploymentConfigs, StatefulSets, ReplicaSets, DaemonSets, CronJobs and Jobs.",
	EnsureStageBackup:               "Creating a stage backup.",
	StageBackupCreated:              "Waiting for stage backup to complete.",
	StageBackupFailed:               "Migration failed during stage backup.",
	EnsureInitialBackupReplicated:   "Waiting for initial Velero backup replication to target cluster.",
	EnsureStageBackupReplicated:     "Waiting for stage Velero backup replication to target cluster.",
	EnsureStageRestore:              "Creating a stage Velero restore including OpenShift resources and PVCs.",
	StageRestoreCreated:             "Waiting for stage Velero restore to complete.",
	StageRestoreFailed:              "Migration failed during stage Velero restore.",
	EnsureFinalRestore:              "Creating final Velero restore.",
	FinalRestoreCreated:             "Waiting for final Velero restore to complete.",
	FinalRestoreFailed:              "Migration failed during final Velero restore.",
	Verification:                    "Verifying health of migrated Pods.",
	Rollback:                        "Starting rollback",
	EnsureStagePodsDeleted:          "Deleting any leftover stage Pods.",
	EnsureStagePodsTerminated:       "Waiting for leftover stage Pod deletion to finish.",
	EnsureAnnotationsDeleted:        "Removing migration annotations and labels from PVs, PVCs, Pods, ImageStreams, and Namespaces. Annotations and labels provide migration instructions to Velero, Velero Plugins and Restic.",
	EnsureMigratedDeleted:           "Rolling back. Waiting for migrated resource deletion.",
	DeleteMigrated:                  "Rolling back. Deleting migrated resources from target cluster.",
	DeleteBackups:                   "Deleting Velero Backups created during migration.",
	DeleteRestores:                  "Deleting Velero Restores created during migration.",
	DeleteHookJobs:                  "Deleting user-defined hook Jobs and Pods created during migration.",
	MigrationFailed:                 "Migration failed.",
	Canceling:                       "Migration cancellation in progress.",
	Canceled:                        "Migration canceled.",
	Completed:                       "Migration completed.",
}
