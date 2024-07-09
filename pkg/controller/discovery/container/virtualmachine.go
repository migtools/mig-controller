package container

import (
	"context"
	"time"

	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// A collection of k8s VirtualMachine resources.
type VirtualMachine struct {
	// Base
	BaseCollection
}

func (r *VirtualMachine) AddWatch(dsController controller.Controller) error {
	err := dsController.Watch(
		source.Kind(r.ds.manager.GetCache(), &virtv1.VirtualMachine{}),
		&handler.EnqueueRequestForObject{},
		r)
	if err != nil {
		sink.Trace(err)
		return err
	}

	return nil
}

func (r *VirtualMachine) Reconcile() error {
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
		"VirtualMachine (collection) reconciled.",
		"ns",
		r.ds.Cluster.Namespace,
		"name",
		r.ds.Cluster.Name,
		"duration",
		time.Since(mark))

	return nil
}

func (r *VirtualMachine) GetDiscovered() ([]model.Model, error) {
	models := []model.Model{}
	onCluster := virtv1.VirtualMachineList{}
	err := r.ds.Client.List(context.TODO(), &onCluster)
	if err != nil {
		sink.Trace(err)
		return nil, err
	}
	for _, discovered := range onCluster.Items {
		vm := &model.VirtualMachine{
			Base: model.Base{
				Cluster: r.ds.Cluster.PK,
			},
		}
		vm.With(&discovered)
		models = append(models, vm)
	}

	return models, nil
}

func (r *VirtualMachine) GetStored() ([]model.Model, error) {
	models := []model.Model{}
	list, err := model.VirtualMachine{
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
	for _, vm := range list {
		models = append(models, vm)
	}

	return models, nil
}

//
// Predicate methods.
//

func (r *VirtualMachine) Create(e event.CreateEvent) bool {
	object, cast := e.Object.(*virtv1.VirtualMachine)
	if !cast {
		return false
	}
	vm := model.VirtualMachine{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	vm.With(object)
	r.ds.Create(&vm)

	return false
}

func (r *VirtualMachine) Update(e event.UpdateEvent) bool {
	object, cast := e.ObjectNew.(*virtv1.VirtualMachine)
	if !cast {
		return false
	}
	vm := model.VirtualMachine{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	vm.With(object)
	r.ds.Update(&vm)

	return false
}

func (r *VirtualMachine) Delete(e event.DeleteEvent) bool {
	object, cast := e.Object.(*virtv1.VirtualMachine)
	if !cast {
		return false
	}
	vm := model.VirtualMachine{
		Base: model.Base{
			Cluster: r.ds.Cluster.PK,
		},
	}
	vm.With(object)
	r.ds.Delete(&vm)

	return false
}

func (r *VirtualMachine) Generic(e event.GenericEvent) bool {
	return false
}
