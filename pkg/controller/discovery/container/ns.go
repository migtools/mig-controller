package container

import (
	"context"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

//
// A collection of k8s Namespace resources.
type NsCollection struct {
	// Base
	BaseCollection
}

func (r *NsCollection) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &v1.Namespace{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *NsCollection) Reconcile() error {
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
		"NsCollection reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *NsCollection) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.NamespaceList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
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

func (r *NsCollection) GetStored() ([]model.Model, error) {
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

func (r *NsCollection) Create(e event.CreateEvent) bool {
	Log.Reset()
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

func (r *NsCollection) Update(e event.UpdateEvent) bool {
	return false
}

func (r *NsCollection) Delete(e event.DeleteEvent) bool {
	Log.Reset()
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

func (r *NsCollection) Generic(e event.GenericEvent) bool {
	return false
}
