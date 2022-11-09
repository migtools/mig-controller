package model

import (
	"encoding/json"
	"k8s.io/api/core/v1"
)

// Pod model
type Pod struct {
	Base
}

// Update the model `with` a k8s Pod.
func (m *Pod) With(object *v1.Pod) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)

}

// Encode the object.
func (m *Pod) EncodeObject(pod *v1.Pod) {
	object, _ := json.Marshal(pod)
	m.Object = string(object)
}

// Decode the object.
func (m *Pod) DecodeObject() *v1.Pod {
	pod := &v1.Pod{}
	json.Unmarshal([]byte(m.Object), pod)
	return pod
}

// Count in the DB.
func (m Pod) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the model from in the DB.
func (m Pod) List(db DB, options ListOptions) ([]*Pod, error) {
	list := []*Pod{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Pod))
	}

	return list, nil
}

// Fetch the model from the DB.
func (m *Pod) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Pod) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Pod) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Pod) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
