package model

import (
	"encoding/json"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

//
// DirectVolume model.
type DirectVolume struct {
	CR
}

//
// Update the model `with` a DirectVolumeMigration.
func (m *DirectVolume) With(object *migapi.DirectVolumeMigration) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *DirectVolume) EncodeObject(dvm *migapi.DirectVolumeMigration) {
	object, _ := json.Marshal(dvm)
	m.Object = string(object)
}

//
// Decode the object.
func (m *DirectVolume) DecodeObject() *migapi.DirectVolumeMigration {
	dvm := &migapi.DirectVolumeMigration{}
	json.Unmarshal([]byte(m.Object), dvm)
	return dvm
}

//
// Count in the DB.
func (m DirectVolume) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m DirectVolume) List(db DB, options ListOptions) ([]*DirectVolume, error) {
	list := []*DirectVolume{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*DirectVolume))
	}

	return list, err
}

//
// Fetch the model from the DB.
func (m *DirectVolume) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *DirectVolume) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *DirectVolume) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *DirectVolume) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

//
// DirectImage model.
type DirectImage struct {
	CR
}

//
// Update the model `with` a DirectImageMigration.
func (m *DirectImage) With(object *migapi.DirectImageMigration) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *DirectImage) EncodeObject(dim *migapi.DirectImageMigration) {
	object, _ := json.Marshal(dim)
	m.Object = string(object)
}

//
// Decode the object.
func (m *DirectImage) DecodeObject() *migapi.DirectImageMigration {
	dim := &migapi.DirectImageMigration{}
	json.Unmarshal([]byte(m.Object), dim)
	return dim
}

//
// Count in the DB.
func (m DirectImage) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the model from the DB.
func (m DirectImage) List(db DB, options ListOptions) ([]*DirectImage, error) {
	list := []*DirectImage{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*DirectImage))
	}

	return list, err
}

//
// Fetch the model from the DB.
func (m *DirectImage) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *DirectImage) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *DirectImage) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *DirectImage) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
