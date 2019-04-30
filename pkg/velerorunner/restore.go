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
	"context"
	"fmt"
	"reflect"

	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunRestore runs a Velero Restore if it hasn't been run already
func RunRestore(c client.Client, vRestoreNew *velerov1.Restore, backupNsName types.NamespacedName, logPrefix string) (*velerov1.Restore, error) {
	restoreNsName := types.NamespacedName{Namespace: vRestoreNew.Namespace, Name: vRestoreNew.Name}
	vRestoreExisting := &velerov1.Restore{}

	err := c.Get(context.TODO(), restoreNsName, vRestoreExisting)
	if err != nil {
		if errors.IsNotFound(err) {
			// Restore not found, we need to check if the Backup we want to use has completed
			vBackupDest := &velerov1.Backup{}
			err = c.Get(context.TODO(), backupNsName, vBackupDest)
			if err != nil {
				if errors.IsNotFound(err) {
					log.Info(fmt.Sprintf("[%s] Velero Backup [%s/%s] doesn't yet exist on destination cluster, waiting...",
						logPrefix, backupNsName.Namespace, backupNsName.Name))
					return nil, nil // don't requeue
				}
			}

			if vBackupDest.Status.Phase != velerov1.BackupPhaseCompleted {
				log.Info(fmt.Sprintf("[%s] Velero Backup [%s/%s] in unusable phase [%s] on destination cluster",
					logPrefix, backupNsName.Namespace, backupNsName.Name, vBackupDest.Status.Phase))
				return nil, fmt.Errorf("Backup phase unusable") // don't requeue
			}

			log.Info(fmt.Sprintf("[%s] Found completed Velero Backup [%s/%s] on destination cluster, creating Restore on destination cluster",
				logPrefix, backupNsName.Namespace, backupNsName.Name))
			// Create a restore once we're certain that the required Backup exists
			err = c.Create(context.TODO(), vRestoreNew)
			if err != nil {
				log.Info(fmt.Sprintf("[%s] Failed to CREATE Velero Restore [%s/%s] on destination cluster",
					logPrefix, vRestoreNew.Namespace, vRestoreNew.Name))
				return nil, err
			}
			log.Info(fmt.Sprintf("[%s] Velero Restore [%s/%s] CREATED successfully on destination cluster",
				logPrefix, vRestoreNew.Namespace, vRestoreNew.Name))
			return vRestoreNew, nil
		}
		return nil, err // requeue
	}

	if !reflect.DeepEqual(vRestoreNew.Spec, vRestoreExisting.Spec) {
		// Send "Create" action for Velero Backup to K8s API
		vRestoreExisting.Spec = vRestoreNew.Spec
		err = c.Update(context.TODO(), vRestoreExisting)
		if err != nil {
			log.Info(fmt.Sprintf("[%s] Failed to UPDATE Velero Restore [%s/%s]",
				logPrefix, vRestoreExisting.Namespace, vRestoreExisting.Name))
			return nil, err
		}
		log.Info(fmt.Sprintf("[%s] Velero Restore [%s/%s] UPDATED successfully on destination cluster",
			logPrefix, vRestoreExisting.Namespace, vRestoreExisting.Name))
		return vRestoreExisting, nil
	}
	log.Info(fmt.Sprintf("[%s] Velero Restore [%s/%s] EXISTS on destination cluster",
		logPrefix, vRestoreNew.Namespace, vRestoreNew.Name))
	return vRestoreExisting, nil
}
