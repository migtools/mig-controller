package model

import (
	"crypto/sha1"
	"database/sql"
	"fmt"
	"github.com/konveyor/mig-controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/settings"
	_ "github.com/mattn/go-sqlite3"
	pathlib "path"
	"reflect"
	"strconv"
	"sync"
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
	statements := []string{
		Pragma,
		ClusterTableDDL,
		NamespaceTableDDL,
		PvTableDDL,
		PodTableDDL,
		PodLabelTableDDL,
		PodLabelIndexDDL,
		PlanTableDDL,
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
	// Set the primary key based on attributes.
	SetPk()
	// Get the model.Base.
	GetBase() *Base
	// Fetch the model from the DB and update the fields.
	// Returns error=`NotFound` when not found.
	Select(DB) error
	// Insert into the DB.
	Insert(DB) error
	// Update in the DB.
	Update(DB) error
	// Delete from the DB.
	Delete(DB) error
}

//
// Base Model
type Base struct {
	// Primary key (digest).
	PK string
	// The k8s resource UID.
	UID string
	// The k8s resourceVersion.
	Version string
	// The k8s resource namespace.
	Namespace string
	// The k8s resource name.
	Name string
	// The (optional) cluster foreign key.
	Cluster string
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
// Get base.
func (m *Base) GetBase() *Base {
	return m
}

// Get `version` as a unit64.
// Returns `0` on parse error.
func (m *Base) IntVersion() uint64 {
	version, err := strconv.ParseUint(m.GetBase().Version, 10, 64)
	if err != nil {
		Log.Trace(err)
		return 0
	}
	return version
}

//
// Fetch the model from the DB.
// Optional for some models.
func (m *Base) Select(db DB) error {
	return NotFound
}

//
// Fetch referenced cluster.
func (m *Base) GetCluster(db DB) (*Cluster, error) {
	cluster := &Cluster{
		Base: Base{
			PK: m.Cluster,
		},
	}
	err := cluster.Select(db)
	return cluster, err
}

//
// Labels
type Labels map[string]string

//
// A label.
type Label struct {
	// Label name.
	Name string
	// Label value.
	Value string
}

//
// Labels used in label queries.
type LabelFilter []Label

func AsLabel(name, value string) Label {
	return Label{Name: name, Value: value}
}
