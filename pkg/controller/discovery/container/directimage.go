package container

import (
	"context"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// A collection of k8s DirectImageMigration resources.
type DirectImageMigration struct {
	// Base
	BaseCollection
}

func (r *DirectImageMigration) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &migapi.DirectImageMigration{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		sink.Trace(err)
		return err
	}

	return nil
}

func (r *DirectImageMigration) Reconcile() error {
	mark := time.Now()
	sr := SimpleReconciler{
		Db: r.ds.Container.Db,
	}
	err := sr.Reconcile(r)
	if err != nil {
		sink.Trace(err)
		return err
	}
	r.hasReconciled = true
	log.Info(
		"DirectImageMigration (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *DirectImageMigration) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.DirectImageMigrationList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		dim := &model.DirectImageMigration{}
		dim.With(&discovered)
		models = append(models, dim)
	}

	return models, nil
}

func (r *DirectImageMigration) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.DirectImageMigration{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, dim := range list {
		models = append(models, dim)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *DirectImageMigration) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*migapi.DirectImageMigration)
	if !cast {
		return false
	}
	dim := model.DirectImageMigration{}
	dim.With(object)
	r.ds.Create(&dim)

	return false
}

func (r *DirectImageMigration) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*migapi.DirectImageMigration)
	if !cast {
		return false
	}
	restore := model.DirectImageMigration{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *DirectImageMigration) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*migapi.DirectImageMigration)
	if !cast {
		return false
	}
	dim := model.DirectImageMigration{}
	dim.With(object)
	r.ds.Delete(&dim)

	return false
}

func (r *DirectImageMigration) Generic(e event.GenericEvent) bool {
	return false
}
