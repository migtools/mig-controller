package container

import (
	"context"
	"time"

	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	batchv1 "k8s.io/api/batch/v1"
)

// A collection of k8s Job resources.
type Job struct {
	// Base
	BaseCollection
}

func (r *Job) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &batchv1.Job{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *Job) Reconcile() error {
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
		"Job (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Job) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := batchv1.JobList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		pvc := &model.Job{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		pvc.With(&discovered)
		models = append(models, pvc)
	}

	return models, nil
}

func (r *Job) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Job{
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

func (r *Job) Create(e event.CreateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*batchv1.Job)
	if !cast {
		return false
	}
	pvc := model.Job{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pvc.With(object)
	r.ds.Create(&pvc)

	return false
}

func (r *Job) Update(e event.UpdateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.ObjectNew.(*batchv1.Job)
	if !cast {
		return false
	}
	pvc := model.Job{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pvc.With(object)
	r.ds.Update(&pvc)

	return false
}

func (r *Job) Delete(e event.DeleteEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*batchv1.Job)
	if !cast {
		return false
	}
	pvc := model.Job{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pvc.With(object)
	r.ds.Delete(&pvc)

	return false
}

func (r *Job) Generic(e event.GenericEvent) bool {
	return false
}
