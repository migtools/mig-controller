package model

import (
	"encoding/json"
	"k8s.io/api/core/v1"
)

//
// Service model.
type Service struct {
	Base
}

//
// Update the model `with` a k8s Service.
func (m *Service) With(object *v1.Service) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *Service) EncodeObject(service *v1.Service) {
	object, _ := json.Marshal(service)
	m.Object = string(object)
}

//
// Encode the object.
func (m *Service) DecodeObject() *v1.Service {
	service := &v1.Service{}
	json.Unmarshal([]byte(m.Object), service)
	return service
}

//
// Fetch the from in the DB.
func (m Service) List(db DB, options ListOptions) ([]*Service, error) {
	list := []*Service{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Service))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *Service) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *Service) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *Service) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *Service) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
