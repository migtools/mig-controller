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

// A collection of k8s DirectVolumeMigration resources.
type DirectVolumeMigration struct {
	// Base
	BaseCollection
}

func (r *DirectVolumeMigration) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &migapi.DirectVolumeMigration{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		sink.Trace(err)
		return err
	}

	return nil
}

func (r *DirectVolumeMigration) Reconcile() error {
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
		"DirectVolumeMigration (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *DirectVolumeMigration) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.DirectVolumeMigrationList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		dv := &model.DirectVolumeMigration{}
		dv.With(&discovered)
		models = append(models, dv)
	}

	return models, nil
}

func (r *DirectVolumeMigration) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.DirectVolumeMigration{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, dv := range list {
		models = append(models, dv)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *DirectVolumeMigration) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*migapi.DirectVolumeMigration)
	if !cast {
		return false
	}
	dv := model.DirectVolumeMigration{}
	dv.With(object)
	r.ds.Create(&dv)

	return false
}

func (r *DirectVolumeMigration) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*migapi.DirectVolumeMigration)
	if !cast {
		return false
	}
	restore := model.DirectVolumeMigration{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *DirectVolumeMigration) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*migapi.DirectVolumeMigration)
	if !cast {
		return false
	}
	dv := model.DirectVolumeMigration{}
	dv.With(object)
	r.ds.Delete(&dv)

	return false
}

func (r *DirectVolumeMigration) Generic(e event.GenericEvent) bool {
	return false
}
