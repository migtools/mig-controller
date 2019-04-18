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
)

// buildVeleroBackup creates a Velero backup with default values for most fields
func buildVeleroBackup(ns string, name string, backupNamespaces []string) *velerov1.Backup {
	backup := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    ns,
		},
		Spec: velerov1.BackupSpec{
			// LabelSelector: &metav1.LabelSelector{
			// 	MatchLabels: map[string]string{"app": "nginx"},
			// },
			StorageLocation:    "default",
			TTL:                metav1.Duration{Duration: 720 * time.Hour},
			IncludedNamespaces: backupNamespaces,
			// Unused but defaulted fields
			ExcludedNamespaces: []string{},
			IncludedResources:  []string{},
			ExcludedResources:  []string{},
			Hooks:              velerov1.BackupHooks{Resources: []velerov1.BackupResourceHookSpec{}},
			// VolumeSnapshotLocations: []string{},
		},
	}
	return backup
}

// buildVeleroRestore creates a mostly blank Velero Restore in a specified ns/name, with the
// ability to specify a unique Velero Backup resource name to restore from.
// TODO: offer more customization
func buildVeleroRestore(ns string, name string, backupUniqueName string) *velerov1.Restore {
	restorePVs := true
	restore := &velerov1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    ns,
		},
		Spec: velerov1.RestoreSpec{
			BackupName: backupUniqueName,
			RestorePVs: &restorePVs,
		},
	}

	return restore
}
