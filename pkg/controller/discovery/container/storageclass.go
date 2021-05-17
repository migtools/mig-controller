package container

import (
	"context"
	"time"

	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	v1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// A collection of k8s StorageClass resources.
type StorageClass struct {
	// Base
	BaseCollection
}

func (r *StorageClass) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &v1.StorageClass{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *StorageClass) Reconcile() error {
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
		"StorageClass (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *StorageClass) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.StorageClassList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		StorageClass := &model.StorageClass{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		StorageClass.With(&discovered)
		models = append(models, StorageClass)
	}

	return models, nil
}

func (r *StorageClass) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.StorageClass{
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
	for _, StorageClass := range list {
		models = append(models, StorageClass)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *StorageClass) Create(e event.CreateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.StorageClass)
	if !cast {
		return false
	}
	StorageClass := model.StorageClass{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	StorageClass.With(object)
	r.ds.Create(&StorageClass)

	return false
}

func (r *StorageClass) Update(e event.UpdateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.ObjectNew.(*v1.StorageClass)
	if !cast {
		return false
	}
	StorageClass := model.StorageClass{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	StorageClass.With(object)
	r.ds.Update(&StorageClass)

	return false
}

func (r *StorageClass) Delete(e event.DeleteEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*v1.StorageClass)
	if !cast {
		return false
	}
	StorageClass := model.StorageClass{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	StorageClass.With(object)
	r.ds.Delete(&StorageClass)

	return false
}

func (r *StorageClass) Generic(e event.GenericEvent) bool {
	return false
}
