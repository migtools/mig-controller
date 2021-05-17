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

// A collection of k8s PV resources.
type PV struct {
	// Base
	BaseCollection
}

func (r *PV) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &v1.PersistentVolume{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *PV) Reconcile() error {
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
		"PV (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *PV) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.PersistentVolumeList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		pv := &model.PV{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		pv.With(&discovered)
		models = append(models, pv)
	}

	return models, nil
}

func (r *PV) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.PV{
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
	for _, pv := range list {
		models = append(models, pv)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *PV) Create(e event.CreateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.PersistentVolume)
	if !cast {
		return false
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pv.With(object)
	r.ds.Create(&pv)

	return false
}

func (r *PV) Update(e event.UpdateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.ObjectNew.(*v1.PersistentVolume)
	if !cast {
		return false
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pv.With(object)
	r.ds.Update(&pv)

	return false
}

func (r *PV) Delete(e event.DeleteEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.PersistentVolume)
	if !cast {
		return false
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pv.With(object)
	r.ds.Delete(&pv)

	return false
}

func (r *PV) Generic(e event.GenericEvent) bool {
	return false
}
