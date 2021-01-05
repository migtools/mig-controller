package container

import (
	"context"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

//
// A collection of k8s DirectVolume resources.
type DirectVolume struct {
	// Base
	BaseCollection
}

func (r *DirectVolume) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &migapi.DirectVolumeMigration{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *DirectVolume) Reconcile() error {
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
		"DirectVolume (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *DirectVolume) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.DirectVolumeMigrationList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		dv := &model.DirectVolume{}
		dv.With(&discovered)
		models = append(models, dv)
	}

	return models, nil
}

func (r *DirectVolume) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.DirectVolume{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
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

func (r *DirectVolume) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*migapi.DirectVolumeMigration)
	if !cast {
		return false
	}
	dv := model.DirectVolume{}
	dv.With(object)
	r.ds.Create(&dv)

	return false
}

func (r *DirectVolume) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*migapi.DirectVolumeMigration)
	if !cast {
		return false
	}
	restore := model.DirectVolume{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *DirectVolume) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*migapi.DirectVolumeMigration)
	if !cast {
		return false
	}
	dv := model.DirectVolume{}
	dv.With(object)
	r.ds.Delete(&dv)

	return false
}

func (r *DirectVolume) Generic(e event.GenericEvent) bool {
	return false
}

//
// A collection of k8s DirectImage resources.
type DirectImage struct {
	// Base
	BaseCollection
}

func (r *DirectImage) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &migapi.DirectImageMigration{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *DirectImage) Reconcile() error {
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
		"DirectImage (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *DirectImage) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.DirectImageMigrationList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		dim := &model.DirectImage{}
		dim.With(&discovered)
		models = append(models, dim)
	}

	return models, nil
}

func (r *DirectImage) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.DirectImage{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		Log.Trace(err)
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

func (r *DirectImage) Create(e event.CreateEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*migapi.DirectImageMigration)
	if !cast {
		return false
	}
	dim := model.DirectImage{}
	dim.With(object)
	r.ds.Create(&dim)

	return false
}

func (r *DirectImage) Update(e event.UpdateEvent) bool {
	Log.Reset()
	object, cast := e.ObjectNew.(*migapi.DirectImageMigration)
	if !cast {
		return false
	}
	restore := model.DirectImage{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *DirectImage) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*migapi.DirectImageMigration)
	if !cast {
		return false
	}
	dim := model.DirectImage{}
	dim.With(object)
	r.ds.Delete(&dim)

	return false
}

func (r *DirectImage) Generic(e event.GenericEvent) bool {
	return false
}
