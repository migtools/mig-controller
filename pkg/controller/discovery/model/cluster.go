package model

import (
	"database/sql"
	"encoding/json"
	"github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/mattn/go-sqlite3"
	"k8s.io/api/core/v1"
	"strings"
)

//
// DDL
var ClusterTableDDL = `
CREATE TABLE IF NOT EXISTS cluster (
  pk        TEXT PRIMARY KEY,
  uid       TEXT NOT NULL,
  version   TEXT NOT NULL,
  namespace TEXT NOT NULL,
  name      TEXT NOT NULL,
  secret    TEXT NOT NULL,
UNIQUE (namespace, name)
);
`

//
// SQL
var ClusterInsertSQL = `
INSERT
INTO cluster (
  pk,
  uid,
  version,
  namespace,
  name,
  secret
)
VALUES (
  :pk,
  :uid,
  :version,
  :namespace,
  :name,
  :secret
);
`

var ClusterGetSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  secret
FROM cluster
WHERE namespace = :namespace AND
      name = :name;
`

var ClusterGetByPkSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  secret
FROM cluster
WHERE pk = :pk
`

var ClusterListSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  secret
FROM cluster
ORDER BY namespace, name
LIMIT :limit OFFSET :offset;
`

var ClusterUpdateSQL = `
UPDATE cluster
SET
  version = :version,
  secret = :secret
WHERE
  pk = :pk;
`

var ClusterDeleteSQL = `
DELETE FROM cluster
WHERE
  namespace = :namespace AND
  name = :name;
`

//
// Cluster model.
type Cluster struct {
	// Base
	Base
	// An associated json-encoded k8s secret.
	Secret string
}

//
// Update the model `with` a MigCluster.
func (m *Cluster) With(object *v1alpha1.MigCluster) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	m.EncodeSecret(object.Spec.ServiceAccountSecretRef)
}

//
// Set the `Secret` field with json encoded ref.
func (m *Cluster) EncodeSecret(ref *v1.ObjectReference) {
	if ref != nil {
		b, _ := json.Marshal(ref)
		m.Secret = string(b)
	}
}

//
// Get the json decoded ref in the `Secret` field.
func (m *Cluster) DecodeSecret() *v1.ObjectReference {
	ref := &v1.ObjectReference{}
	json.Unmarshal([]byte(m.Secret), ref)
	return ref
}

//
// Fetch the model from the DB using either PK
// or the natural keys and update the fields.
func (m *Cluster) Select(db DB) error {
	var row *sql.Row
	var secret string
	if m.PK == "" {
		row = db.QueryRow(
			ClusterGetSQL,
			sql.Named("namespace", m.Namespace),
			sql.Named("name", m.Name))
	} else {
		row = db.QueryRow(
			ClusterGetByPkSQL,
			sql.Named("pk", m.PK))
	}
	err := row.Scan(
		&m.PK,
		&m.UID,
		&m.Version,
		&m.Namespace,
		&m.Name,
		&secret)
	if err != nil && err != sql.ErrNoRows {
		Log.Trace(err)
	}

	return err
}

//
// Insert the model into the DB.
// Update on duplicate key.
func (m *Cluster) Insert(db DB) error {
	m.SetPk()
	r, err := db.Exec(
		ClusterInsertSQL,
		sql.Named("pk", m.PK),
		sql.Named("uid", m.UID),
		sql.Named("version", m.Version),
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name),
		sql.Named("secret", m.Secret))
	if err != nil {
		if sql3Err, cast := err.(sqlite3.Error); cast {
			if sql3Err.Code == sqlite3.ErrConstraint {
				return m.Update(db)
			}
		}
		Log.Trace(err)
		return err
	}
	nRows, err := r.RowsAffected()
	if err != nil {
		Log.Trace(err)
		return err
	}
	if nRows > 0 {
		Log.Info("Cluster inserted.", "model", m.Base)
	}

	return nil
}

//
// Update the model in the DB.
func (m *Cluster) Update(db DB) error {
	m.SetPk()
	r, err := db.Exec(
		ClusterUpdateSQL,
		sql.Named("pk", m.PK),
		sql.Named("version", m.Version),
		sql.Named("secret", m.Secret))
	if err != nil {
		Log.Trace(err)
		return err
	}
	nRows, err := r.RowsAffected()
	if err != nil {
		Log.Trace(err)
		return err
	}
	if nRows > 0 {
		Log.Info("Cluster updated.", "model", m.Base)
	}

	return nil
}

//
// Delete the model in the DB.
func (m *Cluster) Delete(db DB) error {
	m.SetPk()
	r, err := db.Exec(
		ClusterDeleteSQL,
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name))
	if err != nil {
		Log.Trace(err)
		return err
	}
	nRows, err := r.RowsAffected()
	if err != nil {
		Log.Trace(err)
		return err
	}
	if nRows > 0 {
		Log.Info("Cluster deleted.", "model", m.Base)
	}

	return nil
}

//
// List all of the namespaces.
func (m *Cluster) NsList(db DB, page *Page) ([]*Namespace, error) {
	list := []*Namespace{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	cursor, err := db.Query(
		NsListSQL,
		sql.Named("cluster", m.PK),
		sql.Named("limit", page.Limit),
		sql.Named("offset", page.Offset))
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	defer cursor.Close()
	for cursor.Next() {
		ns := Namespace{}
		err := cursor.Scan(
			&ns.PK,
			&ns.UID,
			&ns.Version,
			&ns.Name,
			&ns.Cluster)
		if err != nil {
			Log.Trace(err)
		}
		list = append(list, &ns)
	}

	return list, nil
}

//
// List all of the PVs.
func (m *Cluster) PvList(db DB, page *Page) ([]*PV, error) {
	list := []*PV{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	cursor, err := db.Query(
		PvListSQL,
		sql.Named("cluster", m.PK),
		sql.Named("limit", page.Limit),
		sql.Named("offset", page.Offset))
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	defer cursor.Close()
	for cursor.Next() {
		pv := PV{}
		err := cursor.Scan(
			&pv.PK,
			&pv.UID,
			&pv.Version,
			&pv.Name,
			&pv.Definition,
			&pv.Cluster)
		if err != nil {
			Log.Trace(err)
		}
		list = append(list, &pv)
	}

	return list, nil
}

//
// List all of the pods.
func (m *Cluster) PodList(db DB, page *Page) ([]*Pod, error) {
	list := []*Pod{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	cursor, err := db.Query(
		PodListSQL,
		sql.Named("cluster", m.PK),
		sql.Named("limit", page.Limit),
		sql.Named("offset", page.Offset))
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	defer cursor.Close()
	for cursor.Next() {
		pod := Pod{}
		err := cursor.Scan(
			&pod.PK,
			&pod.UID,
			&pod.Version,
			&pod.Namespace,
			&pod.Name,
			&pod.Definition,
			&pod.Cluster)
		if err != nil {
			Log.Trace(err)
		}
		list = append(list, &pod)
	}

	return list, nil
}

//
// Get a pod by label.
func (m *Cluster) PodListByLabel(db DB, labels LabelFilter, page *Page) ([]*Pod, error) {
	list := []*Pod{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	r := strings.NewReplacer(":pods", PodLabelQuery(labels))
	cursor, err := db.Query(
		r.Replace(ClusterPodListByLabelSQL),
		sql.Named("cluster", m.PK),
		sql.Named("limit", page.Limit),
		sql.Named("offset", page.Offset))
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	defer cursor.Close()
	for cursor.Next() {
		pod := Pod{}
		err := cursor.Scan(
			&pod.PK,
			&pod.UID,
			&pod.Version,
			&pod.Namespace,
			&pod.Name,
			&pod.Definition,
			&pod.Cluster)
		if err != nil {
			Log.Trace(err)
		}
		list = append(list, &pod)
	}

	return list, nil
}

//
// List all clusters.
func ClusterList(db DB, page *Page) ([]Cluster, error) {
	list := []Cluster{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	cursor, err := db.Query(
		ClusterListSQL,
		sql.Named("limit", page.Limit),
		sql.Named("offset", page.Offset))
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	defer cursor.Close()
	cluster := Cluster{}
	for cursor.Next() {
		err := cursor.Scan(
			&cluster.PK,
			&cluster.UID,
			&cluster.Version,
			&cluster.Namespace,
			&cluster.Name,
			&cluster.Secret)
		if err != nil {
			Log.Trace(err)
		}
		list = append(list, cluster)
	}

	return list, nil
}
