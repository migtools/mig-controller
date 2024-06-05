package container

import (
	"context"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

// A collection of k8s Namespace resources.
type Namespace struct {
	// Base
	BaseCollection
}

func (r *Namespace) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(
			r.ds.manager.GetCache(),
			&v1.Namespace{},
		),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *Namespace) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{
		Db: r.ds.Container.Db,
	}
	err := sr.Reconcile(r)
	if err != nil {
		Log.Trace(err)
		return err
	}
	r.hasReconciled = true
	Log.Info(
		0,
		"Namespace (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Namespace) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.NamespaceList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		ns := &model.Namespace{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		ns.With(&discovered)
		models = append(models, ns)
	}

	return models, nil
}

func (r *Namespace) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Namespace{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, ns := range list {
		models = append(models, ns)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *Namespace) Create(e event.CreateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.Namespace)
	if !cast {
		return false
	}
	ns := model.Namespace{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	ns.With(object)
	r.ds.Create(&ns)

	return false
}

func (r *Namespace) Update(e event.UpdateEvent) bool {
	return false
}

func (r *Namespace) Delete(e event.DeleteEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.Namespace)
	if !cast {
		return false
	}
	ns := model.Namespace{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	ns.With(object)
	r.ds.Delete(&ns)

	return false
}

func (r *Namespace) Generic(e event.GenericEvent) bool {
	return false
}
