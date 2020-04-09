package migmigration

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/common"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type MigrationPredicate struct {
	predicate.Funcs
}

func (r MigrationPredicate) Create(e event.CreateEvent) bool {
	if !common.IsInSandboxNamespace(e.Meta.GetNamespace()) {
		return false
	}
	migration, cast := e.Object.(*migapi.MigMigration)
	if cast {
		r.mapRefs(migration)
	}
	return true
}

func (r MigrationPredicate) Update(e event.UpdateEvent) bool {
	if !common.IsInSandboxNamespace(e.MetaNew.GetNamespace()) {
		return false
	}
	old, cast := e.ObjectOld.(*migapi.MigMigration)
	if !cast {
		return true
	}
	new, cast := e.ObjectNew.(*migapi.MigMigration)
	if !cast {
		return true
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
	if !common.IsInSandboxNamespace(e.Meta.GetNamespace()) {
		return false
	}
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
	if !common.IsInSandboxNamespace(e.MetaNew.GetNamespace()) {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigPlan)
	if !cast {
		return false
	}
	// Reconciled by the controller.
	return new.HasReconciled()
}
