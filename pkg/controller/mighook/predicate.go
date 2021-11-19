package mighook

import (
	"reflect"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type HookPredicate struct {
	predicate.Funcs
	Namespace string
}

func (r HookPredicate) Create(e event.CreateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	return true
}

func (r HookPredicate) Update(e event.UpdateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.ObjectNew.GetNamespace() {
		return false
	}
	old, cast := e.ObjectOld.(*migapi.MigHook)
	if !cast {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigHook)
	if !cast {
		return true
	}
	changed := !reflect.DeepEqual(old.Spec, new.Spec)
	return changed
}

func (r HookPredicate) Delete(e event.DeleteEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	return true
}
