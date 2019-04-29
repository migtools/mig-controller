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

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller")

// RunBackup runs a Velero Backup if it hasn't been run already
func RunBackup(c client.Client, backupNsName types.NamespacedName, assets *migapi.MigAssetCollection, logPrefix string, stageBackup bool) (*velerov1.Backup, error) {

	vBackupExisting := &velerov1.Backup{}
	vBackupNew := buildVeleroBackup(backupNsName.Namespace, backupNsName.Name, assets.Spec.Namespaces, stageBackup)

	// Switch over to using a generated name if we are creating
	// if createNew {
	// 	vBackupNew.ObjectMeta.Name = ""
	// 	vBackupNew.ObjectMeta.GenerateName = backupNsName.Name
	// }

	err := c.Get(context.TODO(), backupNsName, vBackupExisting)
	if err != nil {
		if errors.IsNotFound(err) {
			// Backup not found, 'Create'
			err = c.Create(context.TODO(), vBackupNew)
			if err != nil {
				log.Info(fmt.Sprintf("[%s] Failed to CREATE Velero Backup [%s/%s] on source cluster",
					logPrefix, vBackupNew.Namespace, vBackupNew.Name))
				return nil, err
			}
			log.Info(fmt.Sprintf("[%s] Velero Backup [%s/%s] CREATED successfully on source cluster",
				logPrefix, vBackupNew.Namespace, vBackupNew.Name))
			return vBackupNew, nil
		}
		return nil, err
	}

	if !reflect.DeepEqual(vBackupNew.Spec, vBackupExisting.Spec) {
		// Backup doesn't match desired spec, 'Update'
		vBackupExisting.Spec = vBackupNew.Spec
		err = c.Update(context.TODO(), vBackupExisting)
		if err != nil {
			log.Error(err, fmt.Sprintf("[%s] Failed to UPDATE Velero Backup [%s/%s] on src cluster",
				logPrefix, vBackupExisting.Namespace, vBackupExisting.Name))
			return nil, err
		}
		log.Info(fmt.Sprintf("[%s] Velero Backup [%s/%s] UPDATED successfully on source cluster",
			logPrefix, vBackupExisting.Namespace, vBackupExisting.Name))
		return vBackupExisting, nil
	}
	log.Info(fmt.Sprintf("[%s] Velero Backup [%s/%s] EXISTS on source cluster",
		logPrefix, vBackupExisting.Namespace, vBackupExisting.Name))
	return vBackupExisting, nil
}
