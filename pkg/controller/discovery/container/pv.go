package container

import (
	"context"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

// A collection of k8s PV resources.
type PvCollection struct {
	// Base
	BaseCollection
}

func (r *PvCollection) AddWatch(dsController controller.Controller) error {
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

func (r *PvCollection) Reconcile() error {
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
		"PvCollection reconciled.",
		"cluster",
		r.ds.cluster,
		"duration",
		time.Since(mark))

	return nil
}

func (r *PvCollection) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.PersistentVolumeList{}
	err := r.ds.client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		pv := &model.PV{
			Base: model.Base{
				Cluster: r.ds.cluster.PK,
			},
		}
		pv.With(&discovered)
		models = append(models, pv)
	}

	return models, nil
}

func (r *PvCollection) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := r.ds.cluster.PvList(r.ds.Container.Db, nil)
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

func (r *PvCollection) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*v1.PersistentVolume)
	if !cast {
		return false
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: r.ds.cluster.PK,
		},
	}
	pv.With(object)
	err := pv.Insert(r.ds.Container.Db)
	if err != nil {
		Log.Trace(err)
	}

	return false
}

func (r *PvCollection) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*v1.PersistentVolume)
	if !cast {
		return false
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: r.ds.cluster.PK,
		},
	}
	pv.With(object)
	err := pv.Update(r.ds.Container.Db)
	if err != nil {
		Log.Trace(err)
	}

	return false
}

func (r *PvCollection) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*v1.PersistentVolume)
	if !cast {
		return false
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: r.ds.cluster.PK,
		},
	}
	pv.With(object)
	err := pv.Delete(r.ds.Container.Db)
	if err != nil {
		Log.Trace(err)
	}

	return false
}

func (r *PvCollection) Generic(e event.GenericEvent) bool {
	return false
}
