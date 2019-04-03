package migstorage

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type UpdatedPredicate struct {
	predicate.Funcs
}

func (r UpdatedPredicate) Update(e event.UpdateEvent) bool {
	objectOld, cast := e.ObjectOld.(*migapi.MigStorage)
	if !cast {
		return true
	}
	objectNew, cast := e.ObjectNew.(*migapi.MigStorage)
	if !cast {
		return true
	}
	changed := !reflect.DeepEqual(objectOld.Spec, objectNew.Spec)
	return changed
}
