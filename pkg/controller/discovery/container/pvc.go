package container

import (
	"context"
	"time"

	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// A collection of k8s PVC resources.
type PVC struct {
	// Base
	BaseCollection
}

func (r *PVC) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &v1.PersistentVolumeClaim{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *PVC) Reconcile() error {
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
		"PVC (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *PVC) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.PersistentVolumeClaimList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		pvc := &model.PVC{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		pvc.With(&discovered)
		models = append(models, pvc)
	}

	return models, nil
}

func (r *PVC) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.PVC{
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
	for _, pvc := range list {
		models = append(models, pvc)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *PVC) Create(e event.CreateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.PersistentVolumeClaim)
	if !cast {
		return false
	}
	pvc := model.PVC{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pvc.With(object)
	r.ds.Create(&pvc)

	return false
}

func (r *PVC) Update(e event.UpdateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.ObjectNew.(*v1.PersistentVolumeClaim)
	if !cast {
		return false
	}
	pvc := model.PVC{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pvc.With(object)
	r.ds.Update(&pvc)

	return false
}

func (r *PVC) Delete(e event.DeleteEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.PersistentVolumeClaim)
	if !cast {
		return false
	}
	pvc := model.PVC{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pvc.With(object)
	r.ds.Delete(&pvc)

	return false
}

func (r *PVC) Generic(e event.GenericEvent) bool {
	return false
}
