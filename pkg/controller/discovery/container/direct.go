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

//
// A collection of k8s DirectVolumeMigration resources.
type DirectVolumeMigration struct {
	// Base
	BaseCollection
}

func (r *DirectVolumeMigration) AddWatch(dsController controller.Controller) error {
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

func (r *DirectVolumeMigration) Reconcile() error {
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
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
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

func (r *DirectVolumeMigration) Create(e event.CreateEvent) bool {
	Log.Reset()
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
	Log.Reset()
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
	Log.Reset()
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

//
// A collection of k8s DirectImageMigration resources.
type DirectImageMigration struct {
	// Base
	BaseCollection
}

func (r *DirectImageMigration) AddWatch(dsController controller.Controller) error {
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

func (r *DirectImageMigration) Reconcile() error {
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
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
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

func (r *DirectImageMigration) Create(e event.CreateEvent) bool {
	Log.Reset()
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
	Log.Reset()
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
	Log.Reset()
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

//
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
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
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
	Log.Reset()
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
	Log.Reset()
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
	Log.Reset()
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
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
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
	Log.Reset()
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
	Log.Reset()
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
	Log.Reset()
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
