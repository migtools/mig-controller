package model

import (
	"encoding/json"

	virtv1 "kubevirt.io/api/core/v1"
)

// StorageClass model.
type VirtualMachine struct {
	Base
}

// Update the model `with` a k8s StorageClass.
func (m *VirtualMachine) With(object *virtv1.VirtualMachine) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

// Encode the object.
func (m *VirtualMachine) EncodeObject(virtualMachine *virtv1.VirtualMachine) {
	object, _ := json.Marshal(virtualMachine)
	m.Object = string(object)
}

// Encode the object.
func (m *VirtualMachine) DecodeObject() *virtv1.VirtualMachine {
	virtualMachine := &virtv1.VirtualMachine{}
	json.Unmarshal([]byte(m.Object), virtualMachine)
	return virtualMachine
}

// Count in the DB.
func (m VirtualMachine) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the virtual machine list from in the DB.
func (m VirtualMachine) List(db DB, options ListOptions) ([]*VirtualMachine, error) {
	list := []*VirtualMachine{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*VirtualMachine))
	}

	return list, nil
}

// Fetch the model from the DB.
func (m *VirtualMachine) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *VirtualMachine) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *VirtualMachine) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *VirtualMachine) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
