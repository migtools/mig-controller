package container

import (
	"context"
	"time"

	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	v1 "github.com/openshift/api/route/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// A collection of k8s Route resources.
type Route struct {
	// Base
	BaseCollection
}

func (r *Route) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &v1.Route{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		sink.Trace(err)
		return err
	}

	return nil
}

func (r *Route) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{Db: r.ds.Container.Db}
	err := sr.Reconcile(r)
	if err != nil {
		sink.Trace(err)
		return err
	}
	r.hasReconciled = true
	log.Info(
		"Route (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Route) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.RouteList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		ns := &model.Route{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		ns.With(&discovered)
		models = append(models, ns)
	}

	return models, nil
}

func (r *Route) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Route{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, route := range list {
		models = append(models, route)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *Route) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*v1.Route)
	if !cast {
		return false
	}
	route := model.Route{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	route.With(object)
	r.ds.Create(&route)

	return false
}

func (r *Route) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*v1.Route)
	if !cast {
		return false
	}
	route := model.Route{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	route.With(object)
	r.ds.Update(&route)

	return false
}

func (r *Route) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*v1.Route)
	if !cast {
		return false
	}
	route := model.Route{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	route.With(object)
	r.ds.Delete(&route)

	return false
}

func (r *Route) Generic(e event.GenericEvent) bool {
	return false
}
