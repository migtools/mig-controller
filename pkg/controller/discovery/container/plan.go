package container

import (
	"context"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

//
// A collection of k8s Plan resources.
type Plan struct {
	// Base
	BaseCollection
}

func (r *Plan) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &migapi.MigPlan{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
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
		Log.Trace(err)
		return err
	}
	r.hasReconciled = true
	Log.Info(
		"Plan reconciled.",
		"cluster",
		r.ds.Name(),
		"duration",
		time.Since(mark))

	return nil
}

func (r *Plan) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := migapi.MigPlanList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
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
	list, err := model.Plan{}.List(r.ds.Container.Db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
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
	Log.Reset()
	object, cast := e.Object.(*migapi.MigPlan)
	if !cast {
		return false
	}
	plan := &model.Plan{}
	plan.With(object)
	r.ds.Create(plan)

	return false
}

func (r *Plan) Update(e event.UpdateEvent) bool {
	Log.Reset()
	o, cast := e.ObjectOld.(*migapi.MigPlan)
	if !cast {
		return false
	}
	n, cast := e.ObjectNew.(*migapi.MigPlan)
	if !cast {
		return false
	}
	changed := !reflect.DeepEqual(
		o.Spec.SrcMigClusterRef,
		n.Spec.SrcMigClusterRef) ||
		!reflect.DeepEqual(
			o.Spec.DestMigClusterRef,
			n.Spec.DestMigClusterRef)
	if changed {
		plan := &model.Plan{}
		plan.With(n)
		r.ds.Update(plan)
	}

	return false
}

func (r *Plan) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	object, cast := e.Object.(*migapi.MigPlan)
	if !cast {
		return false
	}
	plan := &model.Plan{}
	plan.With(object)
	r.ds.Delete(plan)

	return false
}

func (r *Plan) Generic(e event.GenericEvent) bool {
	return false
}
