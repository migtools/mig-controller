package model

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
)

// Event model.
type Event struct {
	Base
}

// Update the model `with` a k8s Event.
func (m *Event) With(object *corev1.Event) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = Labels{
		"involvedObjectKind":      object.InvolvedObject.Kind,
		"involvedObjectName":      object.InvolvedObject.Name,
		"involvedObjectNamespace": object.InvolvedObject.Namespace,
		"involvedObjectUID":       string(object.InvolvedObject.UID),
	}
	m.EncodeObject(object)
}

// Encode the object.
func (m *Event) EncodeObject(event *corev1.Event) {
	object, _ := json.Marshal(event)
	m.Object = string(object)
}

// Encode the object.
func (m *Event) DecodeObject() *corev1.Event {
	event := &corev1.Event{}
	json.Unmarshal([]byte(m.Object), event)
	return event
}

// Count in the DB.
func (m Event) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the from in the DB.
func (m Event) List(db DB, options ListOptions) ([]*Event, error) {
	list := []*Event{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Event))
	}

	return list, nil
}

// Fetch the model from the DB.
func (m *Event) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Event) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Event) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Event) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
