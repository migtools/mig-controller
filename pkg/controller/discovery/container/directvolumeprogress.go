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

//
// A collection of k8s DirectVolumeMigrationProgress resources.
type DirectVolumeMigrationProgress struct {
	// Base
	BaseCollection
}

func (r *DirectVolumeMigrationProgress) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &migapi.DirectVolumeMigrationProgress{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *DirectVolumeMigrationProgress) Reconcile() error {
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
		"DirectVolumeMigrationProgress (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *DirectVolumeMigrationProgress) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.DirectVolumeMigrationProgressList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		dvmp := &model.DirectVolumeMigrationProgress{}
		dvmp.With(&discovered)
		models = append(models, dvmp)
	}

	return models, nil
}

func (r *DirectVolumeMigrationProgress) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.DirectVolumeMigrationProgress{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, dvmp := range list {
		models = append(models, dvmp)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *DirectVolumeMigrationProgress) Create(e event.CreateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*migapi.DirectVolumeMigrationProgress)
	if !cast {
		return false
	}
	dvmp := model.DirectVolumeMigrationProgress{}
	dvmp.With(object)
	r.ds.Create(&dvmp)

	return false
}

func (r *DirectVolumeMigrationProgress) Update(e event.UpdateEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.ObjectNew.(*migapi.DirectVolumeMigrationProgress)
	if !cast {
		return false
	}
	restore := model.DirectVolumeMigrationProgress{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *DirectVolumeMigrationProgress) Delete(e event.DeleteEvent) bool {
	Log = logging.WithName("discovery")
	object, cast := e.Object.(*migapi.DirectVolumeMigrationProgress)
	if !cast {
		return false
	}
	dvmp := model.DirectVolumeMigrationProgress{}
	dvmp.With(object)
	r.ds.Delete(&dvmp)

	return false
}

func (r *DirectVolumeMigrationProgress) Generic(e event.GenericEvent) bool {
	return false
}
