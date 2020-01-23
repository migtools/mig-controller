package model

import (
	"database/sql"
	"encoding/json"
	"github.com/mattn/go-sqlite3"
	rbac "k8s.io/api/rbac/v1beta1"
)

//
// DDL
var RoleTableDDL = `
CREATE TABLE IF NOT EXISTS role (
  pk        TEXT PRIMARY KEY,
  uid       TEXT NOT NULL,
  version   TEXT NOT NULL,
  namespace TEXT NOT NULL,
  name      TEXT NOT NULL,
  rules     TEXT NOT NULL,
  cluster   TEXT NOT NULL,
UNIQUE (cluster, uid),
UNIQUE (cluster, namespace, name),
FOREIGN KEY (cluster)
  REFERENCES cluster (pk)
  ON DELETE CASCADE
);
`

//
// SQL
var RoleInsertSQL = `
INSERT INTO role (
  pk,
  uid,
  version,
  namespace,
  name,
  rules,
  cluster
)
VALUES (
  :pk,
  :uid,
  :version,
  :namespace,
  :name,
  :rules,
  :cluster
);
`

var RoleUpdateSQL = `
UPDATE role
SET
  version = :version,
  rules = :rules
WHERE
  pk = :pk;
`

var RoleDeleteSQL = `
DELETE FROM role
WHERE
  pk = :pk;
`

var RoleListSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  rules,
  cluster
FROM role
WHERE
  cluster = :cluster
ORDER BY name;
`

var RoleGetSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  rules,
  cluster
FROM role
WHERE
  cluster = :cluster AND
  namespace = :namespace AND
  name = :name
ORDER BY name;
`

//
// Role model.
type Role struct {
	// Base
	Base
	// User name.
	Rules string
}

//
// Update the model `with` a k8s Role.
func (m *Role) With(object *rbac.Role) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	m.EncodeRules(object.Rules)
}

//
// Update the model `with` a k8s ClusterRole.
func (m *Role) With2(object *rbac.ClusterRole) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	m.EncodeRules(object.Rules)
}

//
// Scan the row.
func (m *Role) Scan(row Row) error {
	return row.Scan(
		&m.PK,
		&m.UID,
		&m.Version,
		&m.Namespace,
		&m.Name,
		&m.Rules,
		&m.Cluster)
}

//
// Encode rules.
func (m *Role) EncodeRules(r []rbac.PolicyRule) {
	rules, _ := json.Marshal(r)
	m.Rules = string(rules)
}

//
// Decode rules.
func (m *Role) DecodeRules() []rbac.PolicyRule {
	rules := []rbac.PolicyRule{}
	json.Unmarshal([]byte(m.Rules), &rules)
	return rules
}

//
// Fetch the model from the DB using the natural keys
// and update the fields.
func (m *Role) Select(db DB) error {
	row := db.QueryRow(
		RoleGetSQL,
		sql.Named("cluster", m.Cluster),
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name))
	err := m.Scan(row)
	if err != nil && err != sql.ErrNoRows {
		Log.Trace(err)
	}

	return err
}

//
// Insert the model into the DB.
func (m *Role) Insert(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(
		RoleInsertSQL,
		sql.Named("pk", m.PK),
		sql.Named("uid", m.UID),
		sql.Named("version", m.Version),
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name),
		sql.Named("rules", m.Rules),
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
		Log.Info("Role inserted.", "model", m.Base)
	}

	return nil
}

//
// Update the model in the DB.
func (m *Role) Update(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(
		RoleUpdateSQL,
		sql.Named("version", m.Version),
		sql.Named("rules", m.Rules),
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
		Log.Info("Role updated.", "model", m.Base)
	}

	return nil
}

//
// Delete the model in the DB.
func (m *Role) Delete(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(RoleDeleteSQL, sql.Named("pk", m.PK))
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
		Log.Info("Role deleted.", "model", m.Base)
	}

	return nil
}
