package model

import (
	"encoding/json"

	v1 "k8s.io/api/storage/v1"
)

// StorageClass model.
type StorageClass struct {
	Base
}

// Update the model `with` a k8s StorageClass.
func (m *StorageClass) With(object *v1.StorageClass) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

// Encode the object.
func (m *StorageClass) EncodeObject(StorageClass *v1.StorageClass) {
	object, _ := json.Marshal(StorageClass)
	m.Object = string(object)
}

// Encode the object.
func (m *StorageClass) DecodeObject() *v1.StorageClass {
	StorageClass := &v1.StorageClass{}
	json.Unmarshal([]byte(m.Object), StorageClass)
	return StorageClass
}

// Count in the DB.
func (m StorageClass) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the from in the DB.
func (m StorageClass) List(db DB, options ListOptions) ([]*StorageClass, error) {
	list := []*StorageClass{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*StorageClass))
	}

	return list, nil
}

// Fetch the model from the DB.
func (m *StorageClass) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *StorageClass) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *StorageClass) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *StorageClass) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
