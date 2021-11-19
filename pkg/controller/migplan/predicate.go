package migplan

import (
	"reflect"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type PlanPredicate struct {
	predicate.Funcs
	Namespace string
}

func (r PlanPredicate) Create(e event.CreateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	plan, cast := e.Object.(*migapi.MigPlan)
	if cast {
		r.mapRefs(plan)
	}
	return true
}

func (r PlanPredicate) Update(e event.UpdateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.ObjectNew.GetNamespace() {
		return false
	}
	old, cast := e.ObjectOld.(*migapi.MigPlan)
	if !cast {
		return true
	}
	new, cast := e.ObjectNew.(*migapi.MigPlan)
	if !cast {
		return true
	}
	changed := !reflect.DeepEqual(old.Spec, new.Spec) ||
		!reflect.DeepEqual(old.DeletionTimestamp, new.DeletionTimestamp)
	if changed {
		r.unmapRefs(old)
		r.mapRefs(new)
	}
	return changed
}

func (r PlanPredicate) Delete(e event.DeleteEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	plan, cast := e.Object.(*migapi.MigPlan)
	if cast {
		r.unmapRefs(plan)
	}
	return true
}

func (r PlanPredicate) mapRefs(plan *migapi.MigPlan) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(plan),
		Namespace: plan.Namespace,
		Name:      plan.Name,
	}

	// source cluster
	ref := plan.Spec.SrcMigClusterRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigCluster{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}

	// destination cluster
	ref = plan.Spec.DestMigClusterRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigCluster{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}

	// storage
	ref = plan.Spec.MigStorageRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigStorage{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}

	// hooks
	for _, hook := range plan.Spec.Hooks {
		ref = hook.Reference
		if migref.RefSet(ref) {
			refMap.Add(refOwner, migref.RefTarget{
				Kind:      migref.ToKind(migapi.MigHook{}),
				Namespace: ref.Namespace,
				Name:      ref.Name,
			})
		}
	}

}

func (r PlanPredicate) unmapRefs(plan *migapi.MigPlan) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(plan),
		Namespace: plan.Namespace,
		Name:      plan.Name,
	}

	// source cluster
	ref := plan.Spec.SrcMigClusterRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigCluster{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}

	// destination cluster
	ref = plan.Spec.DestMigClusterRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigCluster{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}

	// storage
	ref = plan.Spec.MigStorageRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigStorage{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}

	// hooks
	for _, hook := range plan.Spec.Hooks {
		ref = hook.Reference
		if migref.RefSet(ref) {
			refMap.Delete(refOwner, migref.RefTarget{
				Kind:      migref.ToKind(migapi.MigHook{}),
				Namespace: ref.Namespace,
				Name:      ref.Name,
			})
		}
	}
}

type ClusterPredicate struct {
	predicate.Funcs
	Namespace string
}

func (r ClusterPredicate) Create(e event.CreateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	return false
}

func (r ClusterPredicate) Update(e event.UpdateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.ObjectNew.GetNamespace() {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigCluster)
	if !cast {
		return false
	}
	// Reconciled by the controller.
	return new.HasReconciled()
}

type HookPredicate struct {
	predicate.Funcs
	Namespace string
}

func (r HookPredicate) Create(e event.CreateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	return false
}

func (r HookPredicate) Update(e event.UpdateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.ObjectNew.GetNamespace() {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigHook)
	if !cast {
		return false
	}
	// Reconciled by the controller.
	return new.HasReconciled()
}

type StoragePredicate struct {
	predicate.Funcs
	Namespace string
}

func (r StoragePredicate) Create(e event.CreateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	return false
}

func (r StoragePredicate) Update(e event.UpdateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.ObjectNew.GetNamespace() {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigStorage)
	if !cast {
		return false
	}
	// Reconciled by the controller.
	return new.HasReconciled()
}

type MigrationPredicate struct {
	predicate.Funcs
	Namespace string
}

func (r MigrationPredicate) Create(e event.CreateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	return false
}

func (r MigrationPredicate) Update(e event.UpdateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.ObjectNew.GetNamespace() {
		return false
	}
	old, cast := e.ObjectOld.(*migapi.MigMigration)
	if !cast {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigMigration)
	if !cast {
		return false
	}
	started := !old.Status.HasCondition(migapi.Running) &&
		new.Status.HasCondition(migapi.Running)
	stopped := old.Status.HasCondition(migapi.Running) &&
		!new.Status.HasCondition(migapi.Running)
	if started || stopped {
		return true
	}

	return false
}
