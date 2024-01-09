package container

import (
	"context"
	"time"

	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// A collection of k8s Pod resources.
type Pod struct {
	// Base
	BaseCollection
}

func (r *Pod) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &corev1.Pod{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		sink.Trace(err)
		return err
	}

	return nil
}

func (r *Pod) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{Db: r.ds.Container.Db}
	err := sr.Reconcile(r)
	if err != nil {
		sink.Trace(err)
		return err
	}
	r.hasReconciled = true
	log.Info(
		"Pod (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Pod) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := corev1.PodList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		sink.Trace(err)
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

func (r *Pod) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Pod{
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
	for _, pod := range list {
		models = append(models, pod)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *Pod) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*corev1.Pod)
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

func (r *Pod) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*corev1.Pod)
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

func (r *Pod) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*corev1.Pod)
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

func (r *Pod) Generic(e event.GenericEvent) bool {
	return false
}
