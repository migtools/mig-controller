package model

import (
	"encoding/json"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Cluster model.
type Cluster struct {
	CR
}

// Update the model `with` a MigCluster.
func (m *Cluster) With(object *migapi.MigCluster) {
	m.UID = string(object.UID)
	m.Version = object.ResourceVersion
	m.Namespace = object.Namespace
	m.Name = object.Name
	m.labels = object.Labels
	m.EncodeObject(object)
}

// Encode the object.
func (m *Cluster) EncodeObject(cluster *migapi.MigCluster) {
	object, _ := json.Marshal(cluster)
	m.Object = string(object)
}

// Decode the object.
func (m *Cluster) DecodeObject() *migapi.MigCluster {
	cluster := &migapi.MigCluster{}
	json.Unmarshal([]byte(m.Object), cluster)
	return cluster
}

// Count in the DB.
func (m Cluster) Count(db DB, options ListOptions) (int64, error) {
	return Table{db}.Count(&m, options)
}

// Fetch the model from the DB.
func (m Cluster) List(db DB, options ListOptions) ([]*Cluster, error) {
	list := []*Cluster{}
	listed, err := Table{db}.List(&m, options)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, intPtr := range listed {
		list = append(list, intPtr.(*Cluster))
	}

	return list, err
}

// Fetch the model from the DB.
func (m *Cluster) Get(db DB) error {
	return Table{db}.Get(m)
}

// Insert the model into the DB.
func (m *Cluster) Insert(db DB) error {
	m.SetPk()
	return Table{db}.Insert(m)
}

// Update the model in the DB.
func (m *Cluster) Update(db DB) error {
	m.SetPk()
	return Table{db}.Update(m)
}

// Delete the model in the DB.
func (m *Cluster) Delete(db DB) error {
	m.SetPk()
	return Table{db}.Delete(m)
}
