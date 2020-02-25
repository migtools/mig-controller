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
// A collection of k8s Pod resources.
type PodCollection struct {
	// Base
	BaseCollection
}

func (r *PodCollection) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &v1.Pod{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *PodCollection) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{Db: r.ds.Container.Db}
	err := sr.Reconcile(r)
	if err != nil {
		Log.Trace(err)
		return err
	}
	r.hasReconciled = true
	Log.Info(
		"PodCollection reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *PodCollection) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := v1.PodList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		ns := &model.Pod{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		ns.With(&discovered)
		models = append(models, ns)
	}

	return models, nil
}

func (r *PodCollection) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := r.ds.Cluster.PodList(r.ds.Container.Db, nil)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, pod := range list {
		models = append(models, pod)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *PodCollection) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*v1.Pod)
	if !cast {
		return false
	}
	pod := model.Pod{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pod.With(object)
	r.ds.Create(&pod)

	return false
}

func (r *PodCollection) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*v1.Pod)
	if !cast {
		return false
	}
	pod := model.Pod{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pod.With(object)
	r.ds.Update(&pod)

	return false
}

func (r *PodCollection) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*v1.Pod)
	if !cast {
		return false
	}
	pod := model.Pod{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	pod.With(object)
	r.ds.Delete(&pod)

	return false
}

func (r *PodCollection) Generic(e event.GenericEvent) bool {
	return false
}
