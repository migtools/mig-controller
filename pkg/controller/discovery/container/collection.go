package container

import (
	"database/sql"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

//
// Disposition.
// Used for collection reconcile and describes whether a resource
// has been `discovered` (exists on the cluster); Is `stored` in the
// inventory; or both.  A `nil` pointer means that the resource
// does not exist.
type Disposition struct {
	// Discovered (exists) on the cluster.
	discovered model.Model
	// Stored (exists) in the DB.
	stored model.Model
}

//
// Collection of resources.
// Each collection loosely corresponds to a k8s resource and
// to a resource in the REST API. It is responsible for setting up
// watches; performing discovery and managing the inventory in the DB.
type Collection interface {
	// Associate with a DataSource.
	// Mainly to support two phase construction.
	Bind(ds *DataSource)
	// Get the associated DataSource.
	GetDs() *DataSource
	// Add k8s watches.
	// Each watch MUST include a predicate that performs the
	// appropriate changes in the DB instead of creating
	// reconcile events.
	AddWatch(c controller.Controller) error
	// Reconcile the cluster and the DB.
	// Discover resources on the cluster modify the DB as needed.
	// Intended to be performed once when the collection is initialized.
	// The term is borrowed from k8s and similar but different.
	Reconcile() error
	// Get whether the collection is ready to be used.
	// Mainly, that it has been fully initialized (reconciled) and
	// protects against partial data sets.
	IsReady() bool
	// Reset the collection to a like-new state.
	// A reset collection is no longer ready and needs to be reconciled again.
	Reset()
	// Get a list of resources discovered on the cluster.
	// Intended to support `Reconcile()`.
	GetDiscovered() ([]model.Model, error)
	// Get a list of resources stored in the DB.
	// Intended to support `Reconcile()`
	GetStored() ([]model.Model, error)
}

//
// Base collection.
// Provides base fields and methods for collections.
type BaseCollection struct {
	// The Collection.Reconcile() has completed.
	hasReconciled bool
	// The associated DataSource.
	ds *DataSource
}

//
// Get whether the collection has reconciled.
func (r *BaseCollection) IsReady() bool {
	return r.hasReconciled
}

//
// Reset `hasReconciled` and association with a DataSource.
func (r *BaseCollection) Reset() {
	r.hasReconciled = false
}

//
// Bind to a DataSource.
func (r *BaseCollection) Bind(ds *DataSource) {
	r.ds = ds
}

//
// Get the associated DataSource.
func (r *BaseCollection) GetDs() *DataSource {
	return r.ds
}

//
// A basic Reconciler.
// The term is borrowed from k8s and is similar but different.
// Using the `GetDiscovered()` and `GetStored()` methods, a disposition
// is constructed and used to update the DB.
type SimpleReconciler struct {
	// A database connection.
	Db *sql.DB
}

//
// Reconcile the resources on the cluster and the collection in the DB.
// Using the `GetDiscovered()` and `GetStored()` methods, a disposition
// is constructed and used to update the DB.
func (r *SimpleReconciler) Reconcile(collection Collection) error {
	dispositions := map[string]*Disposition{}
	stored, err := collection.GetStored()
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range stored {
		dispositions[m.GetBase().PK] = &Disposition{
			stored: m,
		}
	}
	discovered, err := collection.GetDiscovered()
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, m := range discovered {
		m.SetPk()
		collection.GetDs().HasDiscovered(m)
		if dpn, found := dispositions[m.GetBase().PK]; !found {
			dispositions[m.GetBase().PK] = &Disposition{
				discovered: m,
			}
		} else {
			dpn.discovered = m
		}
	}
	model.Mutex.RLock()
	defer model.Mutex.RUnlock()
	tx, err := r.Db.Begin()
	if err != nil {
		Log.Trace(err)
		return err
	}
	defer tx.Rollback()
	// Delete
	for _, dpn := range dispositions {
		if dpn.stored != nil && dpn.discovered == nil {
			err := dpn.stored.Delete(tx)
			if err != nil {
				Log.Trace(err)
				return err
			}
		}
	}
	// Add
	for _, dpn := range dispositions {
		if dpn.discovered != nil && dpn.stored == nil {
			err := dpn.discovered.Insert(tx)
			if err != nil {
				Log.Trace(err)
				return err
			}
		}
	}
	// Update
	for _, dpn := range dispositions {
		if dpn.discovered == nil || dpn.stored == nil {
			continue
		}
		if dpn.discovered.GetBase().Version == dpn.stored.GetBase().Version {
			continue
		}
		err := dpn.discovered.Update(tx)
		if err != nil {
			Log.Trace(err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}
