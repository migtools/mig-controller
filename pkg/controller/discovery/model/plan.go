package model

import (
	"encoding/json"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Plan model
type Plan struct {
	CR
}

// Update the model `with` a MigPlan.
func (m *Plan) With(object *migapi.MigPlan) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

// Encode the object.
func (m *Plan) EncodeObject(plan *migapi.MigPlan) {
	object, _ := json.Marshal(plan)
	m.Object = string(object)
}

// Decode the object.
func (m *Plan) DecodeObject() *migapi.MigPlan {
	plan := &migapi.MigPlan{}
	json.Unmarshal([]byte(m.Object), plan)
	return plan
}

// Count in the DB.
func (m Plan) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the model from the DB.
func (m Plan) List(db DB, options ListOptions) ([]*Plan, error) {
	list := []*Plan{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Plan))
	}

	return list, nil
}

// Fetch the model from the DB.
func (m *Plan) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Plan) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Plan) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Plan) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}

// Migration  model
type Migration struct {
	CR
}

// Update the model `with` a MigMigration.
func (m *Migration) With(object *migapi.MigMigration) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

// Encode the object.
func (m *Migration) EncodeObject(migration *migapi.MigMigration) {
	object, _ := json.Marshal(migration)
	m.Object = string(object)
}

// Decode the object.
func (m *Migration) DecodeObject() *migapi.MigMigration {
	migration := &migapi.MigMigration{}
	json.Unmarshal([]byte(m.Object), migration)
	return migration
}

// Count in the DB.
func (m Migration) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the model from the DB.
func (m Migration) List(db DB, options ListOptions) ([]*Migration, error) {
	list := []*Migration{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Migration))
	}

	return list, nil
}

// Fetch the model from the DB.
func (m *Migration) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Migration) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Migration) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Migration) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
