package model

import (
	"encoding/json"
	"k8s.io/api/core/v1"
)

//
// PV model.
type PV struct {
	Base
}

//
// Update the model `with` a k8s PersistentVolume.
func (m *PV) With(object *v1.PersistentVolume) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *PV) EncodeObject(pv *v1.PersistentVolume) {
	object, _ := json.Marshal(pv)
	m.Object = string(object)
}

//
// Encode the object.
func (m *PV) DecodeObject() *v1.PersistentVolume {
	pv := &v1.PersistentVolume{}
	json.Unmarshal([]byte(m.Object), pv)
	return pv
}

//
// Fetch the from in t
func (m PV) List(db DB, options ListOptions) ([]*PV, error) {
	list := []*PV{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*PV))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *PV) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *PV) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *PV) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *PV) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
