package model

import (
	"encoding/json"

	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

//
// Backup  model
type Backup struct {
	Base
}

//
// Update the model `with` a Backup.
func (m *Backup) With(object *velero.Backup) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *Backup) EncodeObject(backup *velero.Backup) {
	object, _ := json.Marshal(backup)
	m.Object = string(object)
}

//
// Decode the object.
func (m *Backup) DecodeObject() *velero.Backup {
	backup := &velero.Backup{}
	json.Unmarshal([]byte(m.Object), backup)
	return backup
}

//
// Count in the DB.
func (m Backup) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m Backup) List(db DB, options ListOptions) ([]*Backup, error) {
	list := []*Backup{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Backup))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *Backup) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *Backup) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *Backup) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *Backup) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

//
// Restore  model
type Restore struct {
	Base
}

//
// Update the model `with` a Restore.
func (m *Restore) With(object *velero.Restore) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *Restore) EncodeObject(restore *velero.Restore) {
	object, _ := json.Marshal(restore)
	m.Object = string(object)
}

//
// Decode the object.
func (m *Restore) DecodeObject() *velero.Restore {
	restore := &velero.Restore{}
	json.Unmarshal([]byte(m.Object), restore)
	return restore
}

//
// Count in the DB.
func (m Restore) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m Restore) List(db DB, options ListOptions) ([]*Restore, error) {
	list := []*Restore{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Restore))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *Restore) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *Restore) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *Restore) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *Restore) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

//
// Backup  model
type PodVolumeBackup struct {
	Base
}

//
// Update the model `with` a Backup.
func (m *PodVolumeBackup) With(object *velero.PodVolumeBackup) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *PodVolumeBackup) EncodeObject(backup *velero.PodVolumeBackup) {
	object, _ := json.Marshal(backup)
	m.Object = string(object)
}

//
// Decode the object.
func (m *PodVolumeBackup) DecodeObject() *velero.PodVolumeBackup {
	backup := &velero.PodVolumeBackup{}
	json.Unmarshal([]byte(m.Object), backup)
	return backup
}

//
// Count in the DB.
func (m PodVolumeBackup) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m PodVolumeBackup) List(db DB, options ListOptions) ([]*PodVolumeBackup, error) {
	list := []*PodVolumeBackup{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*PodVolumeBackup))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *PodVolumeBackup) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *PodVolumeBackup) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *PodVolumeBackup) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *PodVolumeBackup) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

//
// Restore  model
type PodVolumeRestore struct {
	Base
}

//
// Update the model `with` a Restore.
func (m *PodVolumeRestore) With(object *velero.PodVolumeRestore) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *PodVolumeRestore) EncodeObject(restore *velero.PodVolumeRestore) {
	object, _ := json.Marshal(restore)
	m.Object = string(object)
}

//
// Decode the object.
func (m *PodVolumeRestore) DecodeObject() *velero.PodVolumeRestore {
	restore := &velero.PodVolumeRestore{}
	json.Unmarshal([]byte(m.Object), restore)
	return restore
}

//
// Count in the DB.
func (m PodVolumeRestore) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m PodVolumeRestore) List(db DB, options ListOptions) ([]*PodVolumeRestore, error) {
	list := []*PodVolumeRestore{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*PodVolumeRestore))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *PodVolumeRestore) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *PodVolumeRestore) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *PodVolumeRestore) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *PodVolumeRestore) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
