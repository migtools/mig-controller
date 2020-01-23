package container

import (
	"context"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	rbac "k8s.io/api/rbac/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

//
// A collection of k8s RoleBinding resources.
type RoleBindingCollection struct {
	// Base
	BaseCollection
}

func (r *RoleBindingCollection) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		&source.Kind{
			Type: &rbac.RoleBinding{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}
	err = dsController.Watch(
		&source.Kind{
			Type: &rbac.ClusterRoleBinding{},
		},
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

func (r *RoleBindingCollection) Reconcile() error {
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
		"RoleBindingCollection reconciled.",
		"cluster",
		r.ds.Cluster,
		"duration",
		time.Since(mark))

	return nil
}

func (r *RoleBindingCollection) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := rbac.RoleBindingList{}
	err := r.ds.Client.List(context.TODO(), nil, &onCluster)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		rb := &model.RoleBinding{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		rb.With(&discovered)
		models = append(models, rb)
	}
	onCluster2 := rbac.ClusterRoleBindingList{}
	err = r.ds.Client.List(context.TODO(), nil, &onCluster2)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster2.Items {
		rb := &model.RoleBinding{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		rb.With2(&discovered)
		models = append(models, rb)
	}

	return models, nil
}

func (r *RoleBindingCollection) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := r.ds.Cluster.RoleBindingList(r.ds.Container.Db)
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	for _, rb := range list {
		models = append(models, rb)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *RoleBindingCollection) Create(e event.CreateEvent) bool {
	Log.Reset()
	rb := &model.RoleBinding{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	object, cast := e.Object.(*rbac.RoleBinding)
	if cast {
		rb.With(object)
	} else {
		object, cast := e.Object.(*rbac.ClusterRoleBinding)
		if cast {
			rb.With2(object)
		} else {
			return false
		}
	}
	r.ds.Create(rb)

	return false
}

func (r *RoleBindingCollection) Update(e event.UpdateEvent) bool {
	Log.Reset()
	rb := &model.RoleBinding{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	object, cast := e.ObjectNew.(*rbac.RoleBinding)
	if cast {
		rb.With(object)
	} else {
		object, cast := e.ObjectNew.(*rbac.ClusterRoleBinding)
		if cast {
			rb.With2(object)
		} else {
			return false
		}
	}
	r.ds.Update(rb)

	return false
}

func (r *RoleBindingCollection) Delete(e event.DeleteEvent) bool {
	Log.Reset()
	rb := &model.RoleBinding{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	object, cast := e.Object.(*rbac.RoleBinding)
	if cast {
		rb.With(object)
	} else {
		object, cast := e.Object.(*rbac.ClusterRoleBinding)
		if cast {
			rb.With2(object)
		} else {
			return false
		}
	}
	r.ds.Delete(rb)

	return false
}

func (r *RoleBindingCollection) Generic(e event.GenericEvent) bool {
	return false
}
