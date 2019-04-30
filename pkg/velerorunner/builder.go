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

package velerorunner

import (
	"time"

	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var stageResources = []string{"pods", "persistentvolumes", "persistentvolumeclaims", "imagestreams", "imagestreamtags"}

// BuildVeleroBackup creates a Velero backup with default values for most fields
func BuildVeleroBackup(nsName types.NamespacedName, backupNamespaces []string, stageBackup bool) *velerov1.Backup {
	includedResources := []string{}
	if stageBackup {
		includedResources = stageResources
	}
	backup := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: nsName.Name,
			Namespace:    nsName.Namespace,
		},
		Spec: velerov1.BackupSpec{
			StorageLocation:    "default",
			TTL:                metav1.Duration{Duration: 720 * time.Hour},
			IncludedNamespaces: backupNamespaces,
			// Unused but defaulted fields
			ExcludedNamespaces: []string{},
			IncludedResources:  includedResources,
			ExcludedResources:  []string{},
			Hooks:              velerov1.BackupHooks{Resources: []velerov1.BackupResourceHookSpec{}},
			// VolumeSnapshotLocations: []string{},
		},
	}
	return backup
}

// BuildVeleroRestore creates a mostly blank Velero Restore in a specified ns/name, with the
// ability to specify a unique Velero Backup resource name to restore from.
// TODO: offer more customization
func BuildVeleroRestore(nsName types.NamespacedName, backupUniqueName string) *velerov1.Restore {
	restorePVs := true
	restore := &velerov1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: nsName.Name,
			Namespace:    nsName.Namespace,
		},
		Spec: velerov1.RestoreSpec{
			BackupName: backupUniqueName,
			RestorePVs: &restorePVs,
		},
	}

	return restore
}
