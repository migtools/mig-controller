package model

import (
	"encoding/json"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

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
