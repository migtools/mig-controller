package container

import (
	"context"
	"time"

	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// A collection of k8s DirectImageStreamMigration resources.
type DirectImageStreamMigration struct {
	// Base
	BaseCollection
}

func (r *DirectImageStreamMigration) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &migapi.DirectImageStreamMigration{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *DirectImageStreamMigration) Reconcile() error {
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
		"DirectImageStreamMigration (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *DirectImageStreamMigration) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.DirectImageStreamMigrationList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		dism := &model.DirectImageStreamMigration{}
		dism.With(&discovered)
		models = append(models, dism)
	}

	return models, nil
}

func (r *DirectImageStreamMigration) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.DirectImageStreamMigration{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, dism := range list {
		models = append(models, dism)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *DirectImageStreamMigration) Create(e event.CreateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*migapi.DirectImageStreamMigration)
	if !cast {
		return false
	}
	dim := model.DirectImageStreamMigration{}
	dim.With(object)
	r.ds.Create(&dim)

	return false
}

func (r *DirectImageStreamMigration) Update(e event.UpdateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.ObjectNew.(*migapi.DirectImageStreamMigration)
	if !cast {
		return false
	}
	restore := model.DirectImageStreamMigration{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *DirectImageStreamMigration) Delete(e event.DeleteEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*migapi.DirectImageStreamMigration)
	if !cast {
		return false
	}
	dism := model.DirectImageStreamMigration{}
	dism.With(object)
	r.ds.Delete(&dism)

	return false
}

func (r *DirectImageStreamMigration) Generic(e event.GenericEvent) bool {
	return false
}
