package model

import (
	"encoding/json"

	v1 "github.com/openshift/api/route/v1"
)

// Route model
type Route struct {
	Base
}

// Update the model `with` a k8s Route.
func (m *Route) With(object *v1.Route) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)

}

// Encode the object.
func (m *Route) EncodeObject(pod *v1.Route) {
	object, _ := json.Marshal(pod)
	m.Object = string(object)
}

// Decode the object.
func (m *Route) DecodeObject() *v1.Route {
	pod := &v1.Route{}
	json.Unmarshal([]byte(m.Object), pod)
	return pod
}

// Count in the DB.
func (m Route) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the model from in the DB.
func (m Route) List(db DB, options ListOptions) ([]*Route, error) {
	list := []*Route{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Route))
	}

	return list, nil
}

// Fetch the model from the DB.
func (m *Route) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Route) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Route) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Route) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
