package model

import (
	v1 "k8s.io/api/core/v1"
)

// Namespace model.
type Namespace struct {
	Base
}

// Update the model `with` a k8s Namespace.
func (m *Namespace) With(object *v1.Namespace) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
}

// Count in the DB.
func (m Namespace) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the from in the DB.
func (m Namespace) List(db DB, options ListOptions) ([]*Namespace, error) {
	list := []*Namespace{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Namespace))
	}

	return list, err
}

// Fetch the from in the DB.
func (m *Namespace) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Namespace) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Namespace) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Namespace) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
