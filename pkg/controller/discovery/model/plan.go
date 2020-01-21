package model

import (
	"database/sql"
	"encoding/json"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/mattn/go-sqlite3"
	"k8s.io/api/core/v1"
)

//
// DDL
var PlanTableDDL = `
CREATE TABLE IF NOT EXISTS plan (
  pk          TEXT PRIMARY KEY,
  uid         TEXT NOT NULL,
  version     TEXT NOT NULL,
  namespace   TEXT NOT NULL,
  name        TEXT NOT NULL,
  source      TEXT,
  destination TEXT,
UNIQUE (namespace, name)
);
`

//
// SQL
var PlanInsertSQL = `
INSERT INTO plan (
  pk,
  uid,
  version,
  namespace,
  name,
  source,
  destination
)
VALUES (
  :pk,
  :uid,
  :version,
  :namespace,
  :name,
  :source,
  :destination
);
`

var PlanGetSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  source,
  destination
FROM plan
WHERE
  namespace = :namespace AND
  name = :name;
`

var PlanListSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  source,
  destination
FROM plan
ORDER BY namespace, name
LIMIT :limit OFFSET :offset;
`

var PlanUpdateSQL = `
UPDATE plan
SET
  version = :version,
  source = :source,
  destination = :destination
WHERE
  pk = :pk;
`

var PlanDeleteSQL = `
DELETE FROM plan
WHERE
  pk = :pk;
`

//
// Plan model
type Plan struct {
	// Base
	Base
	// Source cluster json-encoded ref.
	Source string
	// Destination cluster json-encoded ref.
	Destination string
}

//
// Update the model `with` a MigPlan.
func (m *Plan) With(object *migapi.MigPlan) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	m.EncodeSource(object.Spec.SrcMigClusterRef)
	m.EncodeDestination(object.Spec.DestMigClusterRef)
}

//
// Set the `Source` field with json encoded ref.
func (m *Plan) EncodeSource(ref *v1.ObjectReference) {
	if ref == nil {
		return
	}
	b, _ := json.Marshal(ref)
	m.Source = string(b)
}

//
// Get the json decoded ref in the `Source` field.
func (m *Plan) DecodeSource() *v1.ObjectReference {
	ref := &v1.ObjectReference{}
	json.Unmarshal([]byte(m.Source), ref)
	return ref
}

//
// Set the `Destination` field with json encoded ref.
func (m *Plan) EncodeDestination(ref *v1.ObjectReference) {
	if ref == nil {
		return
	}
	b, _ := json.Marshal(ref)
	m.Destination = string(b)
}

//
// Get the json decoded ref in the `Destination` field.
func (m *Plan) DecodeDestination() *v1.ObjectReference {
	ref := &v1.ObjectReference{}
	json.Unmarshal([]byte(m.Destination), ref)
	return ref
}

//
// Fetch the model from the DB using the natural keys
// and update the fields.
func (m *Plan) Select(db DB) error {
	row := db.QueryRow(
		PlanGetSQL,
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name))
	err := row.Scan(
		&m.PK,
		&m.UID,
		&m.Version,
		&m.Namespace,
		&m.Name,
		&m.Source,
		&m.Destination)
	if err != nil && err != sql.ErrNoRows {
		Log.Trace(err)
	}

	return err
}

//
// Insert the model into the DB.
// Update on duplicate key.
func (m *Plan) Insert(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(
		PlanInsertSQL,
		sql.Named("pk", m.PK),
		sql.Named("uid", m.UID),
		sql.Named("version", m.Version),
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name),
		sql.Named("source", m.Source),
		sql.Named("destination", m.Destination))
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
		Log.Info("Plan inserted.", "model", m.Base)
	}

	return nil
}

//
// Update the model in the DB.
func (m *Plan) Update(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(
		PlanUpdateSQL,
		sql.Named("pk", m.PK),
		sql.Named("version", m.Version),
		sql.Named("source", m.Source),
		sql.Named("destination", m.Destination))
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
		Log.Info("Plan updated.", "model", m.Base)
	}

	return nil
}

//
// Delete the model in the DB.
func (m *Plan) Delete(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(PlanDeleteSQL, sql.Named("pk", m.PK))
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
		Log.Info("Plan deleted.", "model", m.Base)
	}

	return nil
}

//
// List all plans.
func PlanList(db DB, page *Page) ([]*Plan, error) {
	list := []*Plan{}
	if page == nil {
		page = &Page{
			Limit:  int(^uint(0) >> 1),
			Offset: 0,
		}
	}
	cursor, err := db.Query(
		PlanListSQL,
		sql.Named("limit", page.Limit),
		sql.Named("offset", page.Offset))
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	defer cursor.Close()
	for cursor.Next() {
		plan := Plan{}
		err := cursor.Scan(
			&plan.PK,
			&plan.UID,
			&plan.Version,
			&plan.Namespace,
			&plan.Name,
			&plan.Source,
			&plan.Destination)
		if err != nil {
			Log.Trace(err)
		}
		list = append(list, &plan)
	}

	return list, nil
}
