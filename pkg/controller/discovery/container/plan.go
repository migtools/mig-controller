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

// A collection of k8s Plan resources.
type Plan struct {
	// Base
	BaseCollection
}

func (r *Plan) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &migapi.MigPlan{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		sink.Trace(err)
		return err
	}

	return nil
}

func (r *Plan) Reconcile() error {
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
		"Plan (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Plan) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.MigPlanList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		plan := &model.Plan{}
		plan.With(&discovered)
		models = append(models, plan)
	}

	return models, nil
}

func (r *Plan) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Plan{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, plan := range list {
		models = append(models, plan)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *Plan) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*migapi.MigPlan)
	if !cast {
		return false
	}
	plan := model.Plan{}
	plan.With(object)
	r.ds.Create(&plan)

	return false
}

func (r *Plan) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*migapi.MigPlan)
	if !cast {
		return false
	}
	restore := model.Plan{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *Plan) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*migapi.MigPlan)
	if !cast {
		return false
	}
	plan := model.Plan{}
	plan.With(object)
	r.ds.Delete(&plan)

	return false
}

func (r *Plan) Generic(e event.GenericEvent) bool {
	return false
}

// A collection of k8s Migration resources.
type Migration struct {
	// Base
	BaseCollection
}

func (r *Migration) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &migapi.MigMigration{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		sink.Trace(err)
		return err
	}

	return nil
}

func (r *Migration) Reconcile() error {
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
		"Migration (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *Migration) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.MigMigrationList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		migration := &model.Migration{}
		migration.With(&discovered)
		models = append(models, migration)
	}

	return models, nil
}

func (r *Migration) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.Migration{}.List(
		r.ds.Container.Db,
		model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, migration := range list {
		models = append(models, migration)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *Migration) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*migapi.MigMigration)
	if !cast {
		return false
	}
	migration := model.Migration{}
	migration.With(object)
	r.ds.Create(&migration)

	return false
}

func (r *Migration) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*migapi.MigMigration)
	if !cast {
		return false
	}
	restore := model.Migration{}
	restore.With(object)
	r.ds.Update(&restore)

	return false
}

func (r *Migration) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*migapi.MigMigration)
	if !cast {
		return false
	}
	migration := model.Migration{}
	migration.With(object)
	r.ds.Delete(&migration)

	return false
}

func (r *Migration) Generic(e event.GenericEvent) bool {
	return false
}
