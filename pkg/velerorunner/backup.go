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
	"github.com/fusor/mig-controller/pkg/util"
	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var clog = logf.Log.WithName("controller")

// RunBackup ...
func RunBackup(c client.Client, backupNsName types.NamespacedName, assets *migapi.MigAssetCollection, logPrefix string) (*velerov1.Backup, error) {

	vBackupExisting := &velerov1.Backup{}
	vBackupNew := util.BuildVeleroBackup(backupNsName.Namespace, backupNsName.Name, assets.Spec.Namespaces)

	err := c.Get(context.TODO(), backupNsName, vBackupExisting)
	if err != nil {
		if errors.IsNotFound(err) {
			// Backup not found, create
			err = c.Create(context.TODO(), vBackupNew)
			if err != nil {
				clog.Info(fmt.Sprintf("[%s] Failed to CREATE Velero Backup on source cluster", logPrefix))
				return nil, err
			}
			clog.Info(fmt.Sprintf("[%s] Velero Backup CREATED successfully on source cluster", logPrefix))
			return vBackupNew, nil
		}
		return nil, err
	}

	if !reflect.DeepEqual(vBackupNew.Spec, vBackupExisting.Spec) {
		// Send "Create" action for Velero Backup to K8s API
		vBackupExisting.Spec = vBackupNew.Spec
		err = c.Update(context.TODO(), vBackupExisting)
		if err != nil {
			clog.Error(err, fmt.Sprintf("[%s] Failed to UPDATE Velero Backup on src cluster", logPrefix))
			return nil, err
		}
		clog.Info(fmt.Sprintf("[%s] Velero Backup UPDATED successfully on source cluster", logPrefix))
		return vBackupExisting, nil
	}
	clog.Info(fmt.Sprintf("[%s] Velero Backup EXISTS on source cluster", logPrefix))
	return vBackupExisting, nil
}
