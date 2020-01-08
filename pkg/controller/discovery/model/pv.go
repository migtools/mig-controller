package model

import (
	"database/sql"
	"encoding/json"
	"github.com/mattn/go-sqlite3"
	"k8s.io/api/core/v1"
)

//
// DDL
var PvTableDDL = `
CREATE TABLE IF NOT EXISTS pv (
  pk         TEXT PRIMARY KEY,
  uid        TEXT NOT NULL,
  version    TEXT NOT NULL,
  name       TEXT NOT NULL,
  definition TEXT NOT NULL,
  cluster    TEXT NOT NULL,
UNIQUE (cluster, name),
FOREIGN KEY (cluster)
  REFERENCES cluster (pk)
  ON DELETE CASCADE
);
`

//
// SQL
var PvGetSQL = `
SELECT
  pk,
  uid,
  version,
  name,
  definition,
  cluster
FROM pv
WHERE
  cluster = :cluster AND
  name = :name;
`

var PvListSQL = `
SELECT
  pk,
  uid,
  version,
  name,
  definition,
  cluster
FROM pv
WHERE
  cluster = :cluster
ORDER BY name
LIMIT :limit OFFSET :offset;
`

var PvInsertSQL = `
INSERT INTO pv (
  pk,
  uid,
  version,
  name,
  definition,
  cluster
)
VALUES (
  :pk,
  :uid,
  :version,
  :name,
  :definition,
  :cluster
);
`

var PvUpdateSQL = `
UPDATE pv
SET    version = :version,
       definition = :definition
WHERE  pk = :pk;
`

var PvDeleteSQL = `
DELETE FROM pv
WHERE
  pk = ;pk;
`

//
// PV model.
type PV struct {
	// Base
	Base
	// The json-encoded k8s PersistentVolume object.
	Definition string
}

//
// Update the model `with` a k8s PersistentVolume.
func (m *PV) With(object *v1.PersistentVolume) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	definition, _ := json.Marshal(object)
	m.Definition = string(definition)
}

//
// Fetch the model from the DB using the natural keys
// and update the fields.
func (m *PV) Select(db DB) error {
	row := db.QueryRow(
		PvGetSQL,
		sql.Named("cluster", m.Cluster),
		sql.Named("name", m.Name))
	err := row.Scan(
		&m.PK,
		&m.UID,
		&m.Version,
		&m.Name,
		&m.Definition,
		&m.Cluster)
	if err != nil && err != sql.ErrNoRows {
		Log.Trace(err)
	}

	return err
}

//
// Insert the model into the DB.
// Update on duplicate key.
func (m *PV) Insert(db DB) error {
	m.SetPk()
	r, err := db.Exec(
		PvInsertSQL,
		sql.Named("pk", m.PK),
		sql.Named("uid", m.UID),
		sql.Named("version", m.Version),
		sql.Named("name", m.Name),
		sql.Named("definition", m.Definition),
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
		Log.Info("PV inserted.", "model", m.Base)
	}

	return nil
}

//
// Update the model in the DB.
func (m *PV) Update(db DB) error {
	m.SetPk()
	r, err := db.Exec(
		PvUpdateSQL,
		sql.Named("version", m.Version),
		sql.Named("definition", m.Definition),
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
		Log.Info("PV updated.", "model", m.Base)
	}

	return nil
}

//
// Delete the model in the DB.
func (m *PV) Delete(db DB) error {
	m.SetPk()
	r, err := db.Exec(PvDeleteSQL, sql.Named("pk", m.PK))
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
		Log.Info("PV deleted.", "model", m.Base)
	}

	return nil
}
