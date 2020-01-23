package model

import (
	"database/sql"
	"encoding/json"
	"github.com/mattn/go-sqlite3"
	rbac "k8s.io/api/rbac/v1beta1"
)

const (
	SubjectSa    = "ServiceAccount"
	SubjectUser  = "User"
	SubjectGroup = "Group"
)

//
// DDL
var RoleBindingTableDDL = `
CREATE TABLE IF NOT EXISTS roleBinding (
  pk        TEXT PRIMARY KEY,
  uid       TEXT NOT NULL,
  version   TEXT NOT NULL,
  namespace TEXT NOT NULL,
  name      TEXT NOT NULL,
  role      TEXT NOT NULL,
  cluster   TEXT NOT NULL,
UNIQUE (cluster, uid),
UNIQUE (cluster, namespace, name),
FOREIGN KEY (cluster)
  REFERENCES cluster (pk)
  ON DELETE CASCADE
);
`

var RoleBindingSubjectTableDDL = `
CREATE TABLE IF NOT EXISTS roleBindingSubject (
  rb        TEXT NOT NULL,
  kind      TEXT NOT NULL,
  namespace TEXT NOT NULL,
  name      TEXT NOT NULL,
UNIQUE (rb, kind, namespace, name) ON CONFLICT IGNORE,
FOREIGN KEY (rb)
  REFERENCES roleBinding (pk)
  ON DELETE CASCADE
);
`

var RoleBindingSubjectIndexDDL = `
CREATE INDEX IF NOT EXISTS roleBindingSubjectIndex
ON roleBindingSubject (kind, namespace, name);
`

//
// SQL

var RoleBindingGetSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  role,
  cluster
FROM roleBinding
WHERE
  cluster = :cluster AND
  namespace = :namespace AND
  name = :name;
`

var RoleBindingInsertSQL = `
INSERT INTO roleBinding (
  pk,
  uid,
  version,
  namespace,
  name,
  role,
  cluster
)
VALUES (
  :pk,
  :uid,
  :version,
  :namespace,
  :name,
  :role,
  :cluster
);
`

var RoleBindingListSQL = `
SELECT
  pk,
  uid,
  version,
  namespace,
  name,
  role,
  cluster
FROM roleBinding a
WHERE
  a.cluster = :cluster;
`

var RoleBindingUpdateSQL = `
UPDATE roleBinding
SET
  version = :version,
  role = :role
WHERE
  pk = :pk;
`

var RoleBindingBySubjectSQL = `
SELECT
  a.pk,
  a.uid,
  a.version,
  a.namespace,
  a.name,
  a.role,
  a.cluster
FROM roleBinding a,
     roleBindingSubject b
WHERE
  a.cluster = :cluster AND
  b.rb = a.pk AND
  b.kind = :kind AND
  b.namespace = :namespace AND
  b.name = :name;
`

var RoleBindingDeleteSQL = `
DELETE FROM roleBinding
WHERE
  pk = :pk;
`

var RoleBindingSubjectInsertSQL = `
INSERT INTO roleBindingSubject (
  rb,
  kind,
  namespace,
  name
)
VALUES (
  :rb,
  :kind,
  :namespace,
  :name
);
`

var RoleBindingSubjectDeleteSQL = `
DELETE FROM roleBindingSubject
WHERE
  rb = :rb;
`

// RoleBinding subject.
//
type Subject struct {
	Kind      string
	Namespace string
	Name      string
}

//
// RoleBinding model.
type RoleBinding struct {
	// Base
	Base
	// Role json-encoded k8s rbac.RoleRef.
	Role string
	// Subjects
	subjects []rbac.Subject
}

//
// Update the model `with` a k8s RoleBinding.
func (m *RoleBinding) With(object *rbac.RoleBinding) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	m.subjects = object.Subjects
	m.EncodeRole(&object.RoleRef)
}

//
// Update the model `with` a k8s ClusterRoleBinding.
func (m *RoleBinding) With2(object *rbac.ClusterRoleBinding) {
	m.Base.UID = string(object.UID)
	m.Base.Version = object.ResourceVersion
	m.Base.Namespace = object.Namespace
	m.Base.Name = object.Name
	m.subjects = object.Subjects
	m.EncodeRole(&object.RoleRef)
}

//
// Scan the row.
func (m *RoleBinding) Scan(row Row) error {
	return row.Scan(
		&m.PK,
		&m.UID,
		&m.Version,
		&m.Namespace,
		&m.Name,
		&m.Role,
		&m.Cluster)
}

//
// Encode roleRef
func (m *RoleBinding) EncodeRole(r *rbac.RoleRef) {
	ref, _ := json.Marshal(r)
	m.Role = string(ref)
}

//
// Encode roleRef
func (m *RoleBinding) DecodeRole() *rbac.RoleRef {
	ref := &rbac.RoleRef{}
	json.Unmarshal([]byte(m.Role), ref)
	return ref
}

//
// Fetch the model from the DB using the natural keys
// and update the fields.
func (m *RoleBinding) Select(db DB) error {
	row := db.QueryRow(
		RoleBindingGetSQL,
		m.Cluster,
		m.Namespace,
		m.Name)
	err := m.Scan(row)
	if err != nil && err != sql.ErrNoRows {
		Log.Trace(err)
	}

	return err
}

//
// Insert the model into the DB.
func (m *RoleBinding) Insert(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(
		RoleBindingInsertSQL,
		sql.Named("pk", m.PK),
		sql.Named("uid", m.UID),
		sql.Named("version", m.Version),
		sql.Named("namespace", m.Namespace),
		sql.Named("name", m.Name),
		sql.Named("role", m.Role),
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
		Log.Info("RoleBinding inserted.", "model", m.Base)
	}

	err = m.addSubjects(db)

	return err
}

//
// Update the model in the DB.
func (m *RoleBinding) Update(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(
		RoleBindingUpdateSQL,
		sql.Named("version", m.Version),
		sql.Named("role", m.Role),
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
		err = m.updateSubjects(db)
		if err != nil {
			Log.Trace(err)
			return err
		}
		Log.Info("RoleBinding updated.", "model", m.Base)
	}

	return nil
}

//
// Delete the model in the DB.
func (m *RoleBinding) Delete(db DB) error {
	m.SetPk()
	Mutex.RLock()
	defer Mutex.RUnlock()
	r, err := db.Exec(RoleBindingDeleteSQL, sql.Named("pk", m.PK))
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
		Log.Info("RoleBinding deleted.", "model", m.Base)
	}

	return nil
}

//
// Fetch the related role.
func (m *RoleBinding) GetRole(db DB) (*Role, error) {
	ref := m.DecodeRole()
	role := Role{}
	if m.Namespace != "" {
		row := db.QueryRow(
			RoleGetSQL,
			sql.Named("cluster", m.Cluster),
			sql.Named("namespace", m.Namespace),
			sql.Named("name", ref.Name))
		err := role.Scan(row)
		if err == nil {
			return &role, nil
		}
		if err != sql.ErrNoRows {
			Log.Trace(err)
			return nil, err
		}
	}
	row := db.QueryRow(
		RoleGetSQL,
		sql.Named("cluster", m.Cluster),
		sql.Named("namespace", ""),
		sql.Named("name", ref.Name))
	err := role.Scan(row)
	if err != nil && err != sql.ErrNoRows {
		Log.Trace(err)
	}

	return &role, err
}

//
// Insert role-binding/subject in the DB.
func (m *RoleBinding) addSubjects(db DB) error {
	for _, subject := range m.subjects {
		_, err := db.Exec(
			RoleBindingSubjectInsertSQL,
			sql.Named("rb", m.PK),
			sql.Named("kind", subject.Kind),
			sql.Named("namespace", subject.Namespace),
			sql.Named("name", subject.Name))
		if err != nil {
			Log.Trace(err)
			return err
		}
	}

	return nil
}

//
// Delete role-binding/users in the DB.
func (m *RoleBinding) deleteSubjects(db DB) error {
	_, err := db.Exec(
		RoleBindingSubjectDeleteSQL,
		sql.Named("rb", m.PK))
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

//
// Update role-binding/subjects in the DB.
// Delete & Insert.
func (m *RoleBinding) updateSubjects(db DB) error {
	err := m.deleteSubjects(db)
	if err != nil {
		Log.Trace(err)
		return err
	}
	err = m.addSubjects(db)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}
