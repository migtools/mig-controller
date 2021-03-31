package model

import (
	"crypto/sha1"
	"database/sql"
	"fmt"
	pathlib "path"
	"reflect"
	"strconv"
	"sync"

	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/settings"
	_ "github.com/mattn/go-sqlite3"
)

// Shared logger.
var Log *logging.Logger

// Application settings.
var Settings = &settings.Settings

// Not found error.
var NotFound = sql.ErrNoRows

//
// DB driver cannot be used for concurrent writes.
// Model methods must use:
//   - Insert()
//   - Update()
//   - Delete()
// And, all transactions.
var Mutex sync.RWMutex

const (
	Pragma = "PRAGMA foreign_keys = ON"
)

//
// Create the schema in the DB.
func Create() (*sql.DB, error) {
	path := pathlib.Join(Settings.WorkingDir, "discovery.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		panic(err)
	}
	statements := []string{Pragma}
	models := []interface{}{
		&Label{},
		&Cluster{},
		&Plan{},
		&Migration{},
		&DirectVolumeMigration{},
		&DirectImageMigration{},
		&Backup{},
		&Restore{},
		&PodVolumeBackup{},
		&PodVolumeRestore{},
		&Namespace{},
		&Service{},
		&Pod{},
		&PV{},
		&PVC{},
		&StorageClass{},
	}
	for _, m := range models {
		ddl, err := Table{}.DDL(m)
		if err != nil {
			panic(err)
		}
		statements = append(statements, ddl...)
	}
	Mutex.RLock()
	defer Mutex.RUnlock()
	for _, ddl := range statements {
		_, err = db.Exec(ddl)
		if err != nil {
			Log.Trace(err)
			db.Close()
			return nil, err
		}
	}

	Log.Info("Database opened.", "path", path)

	return db, nil
}

//
// Database interface.
// Support model methods taking either sql.DB or sql.Tx.
type DB interface {
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
}

//
// Database interface.
// Support model `Scan` taking either sql.Row or sql.Rows.
type Row interface {
	Scan(...interface{}) error
}

//
// Page.
// Support pagination.
type Page struct {
	// The page offset.
	Offset int
	// The number of items per/page.
	Limit int
}

//
// Slice the collection according to the page definition.
// The `collection` must be a pointer to a `Slice` which is
// modified as needed.
func (p *Page) Slice(collection interface{}) {
	v := reflect.ValueOf(collection)
	switch v.Kind() {
	case reflect.Ptr:
		v = v.Elem()
	default:
		return
	}
	switch v.Kind() {
	case reflect.Slice:
		sliced := reflect.MakeSlice(v.Type(), 0, 0)
		for i := 0; i < v.Len(); i++ {
			if i < p.Offset {
				continue
			}
			if sliced.Len() == p.Limit {
				break
			}
			sliced = reflect.Append(sliced, v.Index(i))
		}
		v.Set(sliced)
	}
}

//
// Model
// Each model represents a table in the DB.
type Model interface {
	// Get the primary key.
	Pk() string
	// Set the primary key based on attributes.
	SetPk()
	// Get model meta-data.
	Meta() *Meta
	// Fetch the model from the DB and update the fields.
	// Returns error=`NotFound` when not found.
	Get(DB) error
	// Insert into the DB.
	Insert(DB) error
	// Update in the DB.
	Update(DB) error
	// Delete from the DB.
	Delete(DB) error
	// Get labels.
	Labels() Labels
}

//
// Model meta-data.
type Meta struct {
	// Primary key.
	PK string
	// The k8s resource UID.
	UID string
	// The k8s resourceVersion.
	Version uint64
	// The k8s resource namespace.
	Namespace string
	// The k8s resource name.
	Name string
}

//
// Base Model
type Base struct {
	// Primary key (digest).
	PK string `sql:"pk"`
	// The k8s resource UID.
	UID string `sql:"const,unique(a)"`
	// The k8s resourceVersion.
	Version string `sql:""`
	// The k8s resource namespace.
	Namespace string `sql:"const,unique(b),key"`
	// The k8s resource name.
	Name string `sql:"const,unique(b),key"`
	// The raw json-encoded k8s resource.
	Object string `sql:""`
	// The (optional) cluster foreign key.
	Cluster string `sql:"const,fk:Cluster(pk),unique(a),unique(b),key"`
	// Labels.
	labels Labels
}

// Get the primary key.
func (m *Base) Pk() string {
	return m.PK
}

//
// Set the primary key.
// The PK is calculated as the SHA1 of the:
//   - cluster FK
//   - k8s UID.
func (m *Base) SetPk() {
	h := sha1.New()
	h.Write([]byte(m.Cluster))
	h.Write([]byte(m.UID))
	m.PK = fmt.Sprintf("%x", h.Sum(nil))
}

//
// Get the meta-data.
func (m *Base) Meta() *Meta {
	n, _ := strconv.ParseUint(m.Version, 10, 64)
	return &Meta{
		PK:        m.PK,
		UID:       m.UID,
		Version:   n,
		Namespace: m.Namespace,
		Name:      m.Name,
	}
}

//
// Get labels.
func (m *Base) Labels() Labels {
	return m.labels
}

//
// Fetch referenced cluster.
func (m *Base) GetCluster(db DB) (*Cluster, error) {
	cluster := &Cluster{
		CR: CR{
			PK: m.Cluster,
		},
	}
	err := cluster.Get(db)
	return cluster, err
}

//
// Custom Resource.
type CR struct {
	// Primary key (digest).
	PK string `sql:"pk"`
	// The k8s resource UID.
	UID string `sql:"const,unique(a)"`
	// The k8s resourceVersion.
	Version string `sql:""`
	// The k8s resource namespace.
	Namespace string `sql:"const,unique(b),key"`
	// The k8s resource name.
	Name string `sql:"const,unique(b),key"`
	// The raw json-encoded k8s resource.
	Object string `sql:""`
}

//
// Get the primary key.
func (m *CR) Pk() string {
	return m.PK
}

//
// Set the primary key.
func (m *CR) SetPk() {
	h := sha1.New()
	h.Write([]byte(m.UID))
	m.PK = fmt.Sprintf("%x", h.Sum(nil))
}

//
// Get the model meta-data.
func (m *CR) Meta() *Meta {
	n, _ := strconv.ParseUint(m.Version, 10, 64)
	return &Meta{
		PK:        m.PK,
		UID:       m.UID,
		Version:   n,
		Namespace: m.Namespace,
		Name:      m.Name,
	}
}

//
// Get associated labels.
func (m *CR) Labels() Labels {
	return Labels{}
}
