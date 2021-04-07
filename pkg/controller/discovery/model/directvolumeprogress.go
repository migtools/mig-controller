package model

import (
	"encoding/json"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

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
