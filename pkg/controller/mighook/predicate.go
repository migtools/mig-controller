package mighook

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type HookPredicate struct {
	predicate.Funcs
}

func (r HookPredicate) Create(e event.CreateEvent) bool {
	return true
}

func (r HookPredicate) Update(e event.UpdateEvent) bool {
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
	return true
}
