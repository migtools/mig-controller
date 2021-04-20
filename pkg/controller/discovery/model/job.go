package model

import (
	"encoding/json"

	batchv1 "k8s.io/api/batch/v1"
)

//
// Job model.
type Job struct {
	Base
}

//
// Update the model `with` a k8s Job.
func (m *Job) With(object *batchv1.Job) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

//
// Encode the object.
func (m *Job) EncodeObject(job *batchv1.Job) {
	object, _ := json.Marshal(job)
	m.Object = string(object)
}

//
// Encode the object.
func (m *Job) DecodeObject() *batchv1.Job {
	job := &batchv1.Job{}
	json.Unmarshal([]byte(m.Object), job)
	return job
}

//
// Count in the DB.
func (m Job) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

//
// Fetch the from in the DB.
func (m Job) List(db DB, options ListOptions) ([]*Job, error) {
	list := []*Job{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Job))
	}

	return list, nil
}

//
// Fetch the model from the DB.
func (m *Job) Get(db DB) error {
	return Table{db}.Get(m)
}

//
// Insert the model into the DB.
func (m *Job) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

//
// Update the model in the DB.
func (m *Job) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

//
// Delete the model in the DB.
func (m *Job) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
