package model

import (
	"database/sql"
	"github.com/mattn/go-sqlite3"
	"k8s.io/api/core/v1"
	"strings"
)

//
// DDL
var NamespaceTableDDL = `
CREATE TABLE IF NOT EXISTS namespace (
  pk      TEXT PRIMARY KEY,
  uid     TEXT NOT NULL,
  version TEXT NOT NULL,
  name    TEXT NOT NULL,
  cluster TEXT NOT NULL,
UNIQUE (cluster, uid),
UNIQUE (cluster, name),
FOREIGN KEY (cluster)
  REFERENCES cluster (pk)
  ON DELETE CASCADE
);
`

//
// SQL
var NsInsertSQL = `
INSERT INTO namespace (
  pk,
  uid,
  version,
  name,
  cluster
)
VALUES (
  :pk,
  :uid,
  :version,
  :name,
  :cluster
);
`

var NsListSQL = `
SELECT
  pk,
  uid,
  version,
  name,
  cluster
FROM namespace
WHERE
  cluster = :cluster
ORDER BY name
LIMIT :limit OFFSET :offset;
`

var NsDeleteSQL = `
DELETE FROM namespace
WHERE
  pk = :pk;
`

//
// Namespace model.
type Namespace struct {
	// Base
	Base
}

//
// Update the model `with` a k8s Namespace.
func (m *Namespace) With(object *v1.Namespace) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
}

//
// Insert the model into the DB.
func (m *Namespace) Insert(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(
		NsInsertSQL,
		sql.Named("pk", m.PK),
		sql.Named("uid", m.UID),
		sql.Named("version", m.Version),
		sql.Named("name", m.Name),
		sql.Named("cluster", m.Cluster))
	if err != nil {
		if sql3Err, cast := err.(sqlite3.Error); cast {
			if sql3Err.Code == sqlite3.ErrConstraint {
				return nil
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
		Log.Info("NS inserted.", "model", m.Base)
	}

	return nil
}

//
// Update the model in the DB.
func (m *Namespace) Update(db DB) error {
	return nil
}

//
// Delete the model in the DB.
func (m *Namespace) Delete(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(NsDeleteSQL, sql.Named("pk", m.PK))
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
		Log.Info("NS deleted.", "model", m.Base)
	}

	return nil
}

//
// List the pods in the namespace.
func (m *Namespace) PodList(db DB, page *Page) ([]*Pod, error) {
	list := []*Pod{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	cursor, err := db.Query(
		NsPodListSQL,
		sql.Named("cluster", m.Cluster),
		sql.Named("namespace", m.Name),
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
// Get a pod in the namespace by label.
func (m *Namespace) PodListByLabel(db DB, labels LabelFilter, page *Page) ([]*Pod, error) {
	list := []*Pod{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	r := strings.NewReplacer(":pods", PodLabelQuery(labels))
	cursor, err := db.Query(
		r.Replace(NsPodListByLabelSQL),
		sql.Named("cluster", m.Cluster),
		sql.Named("namespace", m.Name),
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
