package container

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type Collections []Collection

//
// A DataSource corresponds to a MigCluster and is
// responsible for maintaining a k8s:
//   - Manager/controller
//   - REST configuration
//   - Client
// Each contains a set of `Collection`.
type DataSource struct {
	// The associated (owner) container.
	Container *Container
	// The k8s manager/controller `stop` channel.
	StopChannel chan struct{}
	// Collections.
	Collections Collections
	// The REST configuration for the cluster.
	RestCfg *rest.Config
	// The k8s client for the cluster.
	client client.Client
	// The corresponding cluster in the DB.
	cluster model.Cluster
}

//
// Determine if the DataSource is `ready`.
// The DataSource is `ready` when all of the collections are `ready`.
func (r *DataSource) IsReady() bool {
	for _, collection := range r.Collections {
		if !collection.IsReady() {
			return false
		}
	}

	return true
}

//
// Add resource watches.
// Delegated to the collections.
func (r *DataSource) AddWatches(dsController controller.Controller) error {
	for _, collection := range r.Collections {
		err := collection.AddWatch(dsController)
		if err != nil {
			Log.Trace(err)
			return err
		}
	}

	return nil
}

//
// The k8s reconcile loop.
// Implements the k8s Reconciler interface. The DataSource is the reconciler
// for the container k8s manager/controller but is should never be called. The design
// is for watches added by each collection reference a predicate that handles the change
// rather than queuing a reconcile event.
func (r *DataSource) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

//
// Start the DataSource.
//   - Create the cluster in the DB.
//   - Create a k8s client.
//   - Reconcile each collection.
func (r *DataSource) Start(cluster *migapi.MigCluster) error {
	var err error
	mark := time.Now()
	client, err := cluster.GetClient(r.Container.Client)
	if err != nil {
		Log.Trace(err)
		return err
	}
	d1 := time.Since(mark)
	mark = time.Now()
	r.client = client
	r.cluster = model.Cluster{}
	r.cluster.With(cluster)
	err = r.cluster.Insert(r.Container.Db)
	if err != nil {
		Log.Trace(err)
		return err
	}
	for _, collection := range r.Collections {
		collection.Bind(r)
		err = collection.Reconcile()
		if err != nil {
			Log.Trace(err)
			return err
		}
	}

	d2 := time.Since(mark)

	Log.Info(
		"DataSource Started.",
		"cluster",
		r.cluster,
		"connected",
		d1,
		"reconciled",
		d2)

	return nil
}

//
// Stop the DataSource.
// Stop the associated k8s manager/controller and delete all
// of the associated data in the DB. The data should be deleted
// when the DataSource is not being restarted.
func (r *DataSource) Stop(purge bool) {
	close(r.StopChannel)
	if purge {
		r.cluster.Delete(r.Container.Db)
	}
}
