package model

import (
	"database/sql"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"k8s.io/api/core/v1"
	"strings"
)

//
// DDL
var PodTableDDL = `
CREATE TABLE IF NOT EXISTS pod (
  pk        TEXT PRIMARY KEY,
  uid       TEXT NOT NULL,
  version   TEXT NOT NULL,
  namespace TEXT NOT NULL,
  name      TEXT NOT NULL,
  cluster   TEXT NOT NULL,
UNIQUE (cluster, uid),
UNIQUE (cluster, namespace, name),
FOREIGN KEY (cluster)
  REFERENCES cluster (pk)
  ON DELETE CASCADE
);
`

var PodLabelTableDDL = `
CREATE TABLE IF NOT EXISTS podLabel (
  pod   TEXT NOT NULL,
  name  TEXT NOT NULL,
  value TEXT NOT NULL,
UNIQUE (pod, name) ON CONFLICT REPLACE,
FOREIGN KEY (pod)
  REFERENCES pod (pk)
  ON DELETE CASCADE
);
`

var PodLabelIndexDDL = `
CREATE INDEX IF NOT EXISTS podLabelNameIndex
ON podLabel (
  name,
  value
);
`

//
// SQL
var PodInsertSQL = `
INSERT INTO pod (
  pk,
  uid,
  version,
  namespace,
  name,
  cluster
)
VALUES (
  :pk,
  :uid,
  :version,
  :namespace,
  :name,
  :cluster
);
`

var PodUpdateSQL = `
UPDATE pod
SET
  version = :version
WHERE
  pk = :pk;
`

var PodListSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  cluster
FROM pod
WHERE
  cluster = :cluster
ORDER BY namespace, name
LIMIT :limit OFFSET :offset;
`

var PodListByLabelSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  cluster
FROM pod
WHERE
  pk in ( :pods )
ORDER BY namespace, name
LIMIT :limit OFFSET :offset;
`

var ClusterPodListByLabelSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  cluster
FROM pod
WHERE
  cluster = :cluster AND
  pk in ( :pods )
ORDER BY namespace, name
LIMIT :limit OFFSET :offset;
`

var NsPodListSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  cluster
FROM pod
WHERE
  cluster = :cluster AND
  namespace = :namespace
ORDER BY namespace, name
LIMIT :limit OFFSET :offset;
`

var PodGetSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  cluster
FROM pod
WHERE
  cluster = :cluster AND
  namespace = :namespace AND
  name = :name;
`

var PodDeleteSQL = `
DELETE FROM pod
WHERE
  pk = :pk;
`

var PodLabelInsertSQL = `
INSERT INTO podlabel (
  pod,
  name,
  value
)
VALUES (
  :pod,
  :name,
  :value
);
`

var PodLabelDeleteSQL = `
DELETE FROM podlabel
WHERE
  pod = :pod;
`

var PodLabelTemplate = `
SELECT pod
FROM podlabel
WHERE
  name = '%s' AND
  value = '%s'
`

var NsPodListByLabelSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  cluster
FROM pod
WHERE
  cluster = :cluster AND
  namespace = :namespace AND
  pk in ( :pods )
ORDER BY namespace, name
LIMIT :limit OFFSET :offset;
`

//
// Pod model
type Pod struct {
	// Base
	Base
	// Pod labels.
	labels Labels
}

//
// Update the model `with` a k8s Pod.
func (m *Pod) With(object *v1.Pod) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	m.labels = object.Labels
}

//
// Fetch the model from the DB using the natural keys
// and update the fields.
func (m *Pod) Select(db DB) error {
	row := db.QueryRow(
		PodGetSQL,
		m.Cluster,
		m.Namespace,
		m.Name)
	err := row.Scan(
		&m.PK,
		&m.UID,
		&m.Version,
		&m.Namespace,
		&m.Name,
		&m.Cluster)
	if err != nil && err != sql.ErrNoRows {
		Log.Trace(err)
	}

	return err
}

//
// Insert the model into the DB.
// Update on duplicate key.
func (m *Pod) Insert(db DB) error {
	m.SetPk()
	r, err := db.Exec(
		PodInsertSQL,
		sql.Named("pk", m.PK),
		sql.Named("uid", m.UID),
		sql.Named("version", m.Version),
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name),
		sql.Named("cluster", m.Cluster))
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
		Log.Info("Pod inserted.", "model", m.Base)
	}
	for ln, lv := range m.labels {
		_, err = db.Exec(
			PodLabelInsertSQL,
			sql.Named("pod", m.PK),
			sql.Named("name", ln),
			sql.Named("value", lv))
		if err != nil {
			Log.Trace(err)
			return err
		}
	}

	return nil
}

//
// Update the model in the DB.
func (m *Pod) Update(db DB) error {
	m.SetPk()
	r, err := db.Exec(
		PodUpdateSQL,
		sql.Named("version", m.Version),
		sql.Named("pk", m.PK))
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
		Log.Info("Pod updated.", "model", m.Base)
		err := m.updateLabels(db)
		if err != nil {
			Log.Trace(err)
			return err
		}
	}

	return nil
}

//
// Delete the model in the DB.
func (m *Pod) Delete(db DB) error {
	m.SetPk()
	r, err := db.Exec(PodDeleteSQL, sql.Named("pk", m.PK))
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
		Log.Info("Pod deleted.", "model", m.Base)
	}

	return nil
}

//
// Insert labels in the DB.
func (m *Pod) addLabels(db DB) error {
	for ln, lv := range m.labels {
		_, err := db.Exec(
			PodLabelInsertSQL,
			sql.Named("pod", m.PK),
			sql.Named("name", ln),
			sql.Named("value", lv))
		if err != nil {
			Log.Trace(err)
			return err
		}
	}

	return nil
}

//
// Delete labels in the DB.
func (m *Pod) deleteLabels(db DB) error {
	_, err := db.Exec(
		PodLabelDeleteSQL,
		sql.Named("pod", m.PK))
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

//
// Update labels in the DB.
// Delete & Insert.
func (m *Pod) updateLabels(db DB) error {
	err := m.deleteLabels(db)
	if err != nil {
		Log.Trace(err)
		return err
	}
	err = m.addLabels(db)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

//
// Build Pod label (sub)query.
func PodLabelQuery(labels LabelFilter) string {
	list := []string{}
	for _, l := range labels {
		q := fmt.Sprintf(PodLabelTemplate, l.Name, l.Value)
		list = append(list, q)
	}

	return strings.Join(list, "\nINTERSECT\n")
}

//
// Build Pod list by label (sub)query.
func PodListByLabel(db DB, labels LabelFilter, page *Page) ([]*Pod, error) {
	list := []*Pod{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	r := strings.NewReplacer(":pods", PodLabelQuery(labels))
	cursor, err := db.Query(
		r.Replace(PodListByLabelSQL),
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
			&pod.Cluster)
		if err != nil {
			Log.Trace(err)
		}
		list = append(list, &pod)
	}

	return list, nil
}
