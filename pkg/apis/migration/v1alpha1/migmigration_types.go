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

package v1alpha1

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/fusor/mig-controller/pkg/util"
	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var clog = logf.Log.WithName("controller")

// MigMigrationSpec defines the desired state of MigMigration
type MigMigrationSpec struct {
	MigPlanRef *kapi.ObjectReference `json:"migPlanRef,omitempty"`
}

// MigMigrationStatus defines the observed state of MigMigration
type MigMigrationStatus struct {
	Conditions

	MigrationRunning   bool `json:"migrationStarted,omitempty"`
	MigrationCompleted bool `json:"migrationCompleted,omitempty"`

	StartTimestamp      *metav1.Time `json:"startTimestamp,omitempty"`
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	SrcBackupRef   *kapi.ObjectReference `json:"srcBackupRef,omitempty"`
	DestRestoreRef *kapi.ObjectReference `json:"destRestoreRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigMigration is the Schema for the migmigrations API
// +k8s:openapi-gen=true
type MigMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigMigrationSpec   `json:"spec,omitempty"`
	Status MigMigrationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigMigrationList contains a list of MigMigration
type MigMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigMigration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigMigration{}, &MigMigrationList{})
}

// GetPlan - Get the migration plan.
// Returns `nil` when the reference cannot be resolved.
func (r *MigMigration) GetPlan(client k8sclient.Client) (*MigPlan, error) {
	return GetPlan(client, r.Spec.MigPlanRef)
}

// MarkAsRunning marks the MigMigration status as 'Running'
func (r *MigMigration) MarkAsRunning(c k8sclient.Client) error {
	if r.Status.MigrationCompleted == true || r.Status.MigrationRunning == true {
		return nil
	}
	r.Status.MigrationRunning = true
	r.Status.MigrationCompleted = false
	r.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	err := c.Update(context.TODO(), r)
	if err != nil {
		clog.Info("[mMigration] Failed to mark MigMigration [%s/%s] as running", r.Namespace, r.Name)
		return err
	}
	clog.Info(fmt.Sprintf("[mMigration] Started MigMigration [%s/%s]", r.Namespace, r.Name))
	return nil
}

// MarkAsCompleted marks the MigMigration status as 'Completed'
func (r *MigMigration) MarkAsCompleted(c k8sclient.Client) error {
	if r.Status.MigrationRunning == false || r.Status.MigrationCompleted == true {
		return nil
	}
	r.Status.MigrationRunning = false
	r.Status.MigrationCompleted = true
	r.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
	err := c.Update(context.TODO(), r)
	if err != nil {
		clog.Info("[mMigration] Failed to mark MigMigration [%s/%s] as completed", r.Namespace, r.Name)
		return err
	}
	clog.Info(fmt.Sprintf("[mMigration] Finished MigMigration [%s/%s]", r.Namespace, r.Name))
	return nil
}

// EnsureBackupExists creates a Velero Backup if one doesn't already exist
func (r *MigMigration) EnsureBackupExists(c k8sclient.Client, backupNsName types.NamespacedName, assets *MigAssetCollection) (*velerov1.Backup, error) {

	vBackupExisting := &velerov1.Backup{}
	vBackupNew := util.BuildVeleroBackup(backupNsName.Namespace, backupNsName.Name, assets.Spec.Namespaces)

	err := c.Get(context.TODO(), backupNsName, vBackupExisting)
	if err != nil {
		if errors.IsNotFound(err) {
			// Backup not found, create
			err = c.Create(context.TODO(), vBackupNew)
			if err != nil {
				clog.Info("[mMigration] Failed to CREATE Velero Backup on source cluster")
				return nil, err
			}
			clog.Info("[mMigration] Velero Backup CREATED successfully on source cluster")
			return vBackupNew, nil
		}
		return nil, err
	}

	if !reflect.DeepEqual(vBackupNew.Spec, vBackupExisting.Spec) {
		// Send "Create" action for Velero Backup to K8s API
		vBackupExisting.Spec = vBackupNew.Spec
		err = c.Update(context.TODO(), vBackupExisting)
		if err != nil {
			clog.Error(err, "[mMigration] Failed to UPDATE Velero Backup on src cluster")
			return nil, err
		}
		clog.Info("[mMigration] Velero Backup UPDATED successfully on source cluster")
		return vBackupExisting, nil
	}
	clog.Info("[mMigration] Velero Backup EXISTS on source cluster")
	return vBackupExisting, nil
}
