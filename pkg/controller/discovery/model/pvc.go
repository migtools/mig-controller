package model

import (
	"encoding/json"
	"k8s.io/api/core/v1"
)

//
// PVC model.
type PVC struct {
	Base
}

//
// Update the model `with` a k8s PVC.
func (m *PVC) With(object *v1.PersistentVolumeClaim) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *PVC) EncodeObject(pvc *v1.PersistentVolumeClaim) {
	object, _ := json.Marshal(pvc)
	m.Object = string(object)
}

//
// Encode the object.
func (m *PVC) DecodeObject() *v1.PersistentVolumeClaim {
	pvc := &v1.PersistentVolumeClaim{}
	json.Unmarshal([]byte(m.Object), pvc)
	return pvc
}

//
// Count in the DB.
func (m PVC) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the from in the DB.
func (m PVC) List(db DB, options ListOptions) ([]*PVC, error) {
	list := []*PVC{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*PVC))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *PVC) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *PVC) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *PVC) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *PVC) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
