package migstage

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type StagePredicate struct {
	predicate.Funcs
}

func (r StagePredicate) Create(e event.CreateEvent) bool {
	stage, cast := e.Object.(*migapi.MigStage)
	if cast {
		r.mapRefs(stage)
	}
	return true
}

func (r StagePredicate) Update(e event.UpdateEvent) bool {
	old, cast := e.ObjectOld.(*migapi.MigStage)
	if !cast {
		return true
	}
	new, cast := e.ObjectNew.(*migapi.MigStage)
	if !cast {
		return true
	}
	changed := !reflect.DeepEqual(old.Spec, new.Spec)
	if changed {
		r.unmapRefs(old)
		r.mapRefs(new)
	}
	return changed
}

func (r StagePredicate) Delete(e event.DeleteEvent) bool {
	stage, cast := e.Object.(*migapi.MigStage)
	if cast {
		r.unmapRefs(stage)
	}
	return true
}

func (r StagePredicate) mapRefs(stage *migapi.MigStage) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(stage),
		Namespace: stage.Namespace,
		Name:      stage.Name,
	}

	// plan
	ref := stage.Spec.MigPlanRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigPlan{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}

func (r StagePredicate) unmapRefs(stage *migapi.MigStage) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(stage),
		Namespace: stage.Namespace,
		Name:      stage.Name,
	}

	// plan
	ref := stage.Spec.MigPlanRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(migapi.MigPlan{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}
