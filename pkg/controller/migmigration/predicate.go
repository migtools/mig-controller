package migmigration

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type MigrationPredicate struct {
	predicate.Funcs
}

func (r MigrationPredicate) Create(e event.CreateEvent) bool {
	migration, cast := e.Object.(*migapi.MigMigration)
	if cast {
		r.mapRefs(migration)
	}
	return true
}

func (r MigrationPredicate) Update(e event.UpdateEvent) bool {
	old, cast := e.ObjectOld.(*migapi.MigMigration)
	if !cast {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigMigration)
	if !cast {
		return false
	}
	changed := !reflect.DeepEqual(old.Spec, new.Spec) ||
		(old.Status.HasCondition(HasFinalMigration) &&
			!new.Status.HasCondition(HasFinalMigration))
	if changed {
		r.unmapRefs(old)
		r.mapRefs(new)
	}
	return changed
}

func (r MigrationPredicate) Delete(e event.DeleteEvent) bool {
	migration, cast := e.Object.(*migapi.MigMigration)
	if cast {
		r.unmapRefs(migration)
	}
	return true
}

func (r MigrationPredicate) mapRefs(migration *migapi.MigMigration) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(migration),
		Namespace: migration.Namespace,
		Name:      migration.Name,
	}

	// plan
	ref := migration.Spec.MigPlanRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigPlan{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}

func (r MigrationPredicate) unmapRefs(migration *migapi.MigMigration) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(migration),
		Namespace: migration.Namespace,
		Name:      migration.Name,
	}

	// plan
	ref := migration.Spec.MigPlanRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigPlan{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}

type PlanPredicate struct {
	predicate.Funcs
}

func (r PlanPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (r PlanPredicate) Update(e event.UpdateEvent) bool {
	new, cast := e.ObjectNew.(*migapi.MigPlan)
	if cast {
		// Reconciled by the controller.
		return new.HasReconciled()
	}
	return false
}

func (r PlanPredicate) Delete(e event.DeleteEvent) bool {
	_, cast := e.Object.(*migapi.MigPlan)
	return cast
}

func (r PlanPredicate) Generic(e event.GenericEvent) bool {
	_, cast := e.Object.(*migapi.MigPlan)
	return cast
}
