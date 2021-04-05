package model

import (
	"encoding/json"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

//
// DirectVolumeMigration model.
type DirectVolumeMigration struct {
	CR
}

//
// Update the model `with` a DirectVolumeMigration.
func (m *DirectVolumeMigration) With(object *migapi.DirectVolumeMigration) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *DirectVolumeMigration) EncodeObject(dvm *migapi.DirectVolumeMigration) {
	object, _ := json.Marshal(dvm)
	m.Object = string(object)
}

//
// Decode the object.
func (m *DirectVolumeMigration) DecodeObject() *migapi.DirectVolumeMigration {
	dvm := &migapi.DirectVolumeMigration{}
	json.Unmarshal([]byte(m.Object), dvm)
	return dvm
}

//
// Count in the DB.
func (m DirectVolumeMigration) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m DirectVolumeMigration) List(db DB, options ListOptions) ([]*DirectVolumeMigration, error) {
	list := []*DirectVolumeMigration{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*DirectVolumeMigration))
	}

	return list, err
}

//
// Fetch the model from the DB.
func (m *DirectVolumeMigration) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *DirectVolumeMigration) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *DirectVolumeMigration) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *DirectVolumeMigration) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

//
// DirectImageMigration model.
type DirectImageMigration struct {
	CR
}

//
// Update the model `with` a DirectImageMigration.
func (m *DirectImageMigration) With(object *migapi.DirectImageMigration) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *DirectImageMigration) EncodeObject(dim *migapi.DirectImageMigration) {
	object, _ := json.Marshal(dim)
	m.Object = string(object)
}

//
// Decode the object.
func (m *DirectImageMigration) DecodeObject() *migapi.DirectImageMigration {
	dim := &migapi.DirectImageMigration{}
	json.Unmarshal([]byte(m.Object), dim)
	return dim
}

//
// Count in the DB.
func (m DirectImageMigration) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m DirectImageMigration) List(db DB, options ListOptions) ([]*DirectImageMigration, error) {
	list := []*DirectImageMigration{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*DirectImageMigration))
	}

	return list, err
}

//
// Fetch the model from the DB.
func (m *DirectImageMigration) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *DirectImageMigration) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *DirectImageMigration) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *DirectImageMigration) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

//
// DirectImageStreamMigration model.
type DirectImageStreamMigration struct {
	CR
}

//
// Update the model `with` a DirectImageMigration.
func (m *DirectImageStreamMigration) With(object *migapi.DirectImageStreamMigration) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *DirectImageStreamMigration) EncodeObject(dim *migapi.DirectImageStreamMigration) {
	object, _ := json.Marshal(dim)
	m.Object = string(object)
}

//
// Decode the object.
func (m *DirectImageStreamMigration) DecodeObject() *migapi.DirectImageStreamMigration {
	dism := &migapi.DirectImageStreamMigration{}
	json.Unmarshal([]byte(m.Object), dism)
	return dism
}

//
// Count in the DB.
func (m DirectImageStreamMigration) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m DirectImageStreamMigration) List(db DB, options ListOptions) ([]*DirectImageStreamMigration, error) {
	list := []*DirectImageStreamMigration{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*DirectImageStreamMigration))
	}

	return list, err
}

//
// Fetch the model from the DB.
func (m *DirectImageStreamMigration) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *DirectImageStreamMigration) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *DirectImageStreamMigration) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *DirectImageStreamMigration) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

//
// DirectVolumeMigrationProgress model.
type DirectVolumeMigrationProgress struct {
	CR
}

//
// Update the model `with` a DirectImageMigration.
func (m *DirectVolumeMigrationProgress) With(object *migapi.DirectVolumeMigrationProgress) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *DirectVolumeMigrationProgress) EncodeObject(dvmp *migapi.DirectVolumeMigrationProgress) {
	object, _ := json.Marshal(dvmp)
	m.Object = string(object)
}

//
// Decode the object.
func (m *DirectVolumeMigrationProgress) DecodeObject() *migapi.DirectVolumeMigrationProgress {
	dvmp := &migapi.DirectVolumeMigrationProgress{}
	json.Unmarshal([]byte(m.Object), dvmp)
	return dvmp
}

//
// Count in the DB.
func (m DirectVolumeMigrationProgress) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m DirectVolumeMigrationProgress) List(db DB, options ListOptions) ([]*DirectVolumeMigrationProgress, error) {
	list := []*DirectVolumeMigrationProgress{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*DirectVolumeMigrationProgress))
	}

	return list, err
}

//
// Fetch the model from the DB.
func (m *DirectVolumeMigrationProgress) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *DirectVolumeMigrationProgress) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *DirectVolumeMigrationProgress) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *DirectVolumeMigrationProgress) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
