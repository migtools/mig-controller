package model

import (
	"encoding/json"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Hook model.
type Hook struct {
	CR
}

// Update the model `with` a Hook.
func (m *Hook) With(object *migapi.MigHook) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

// Encode the object.
func (m *Hook) EncodeObject(dim *migapi.MigHook) {
	object, _ := json.Marshal(dim)
	m.Object = string(object)
}

// Decode the object.
func (m *Hook) DecodeObject() *migapi.MigHook {
	hook := &migapi.MigHook{}
	json.Unmarshal([]byte(m.Object), hook)
	return hook
}

// Count in the DB.
func (m Hook) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the model from the DB.
func (m Hook) List(db DB, options ListOptions) ([]*Hook, error) {
	list := []*Hook{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Hook))
	}

	return list, err
}

// Fetch the model from the DB.
func (m *Hook) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Hook) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Hook) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Hook) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
