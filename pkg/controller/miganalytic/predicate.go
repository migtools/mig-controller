package miganalytic

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type AnalyticPredicate struct {
	predicate.Funcs
}

func (r AnalyticPredicate) Create(e event.CreateEvent) bool {
	return true
}

func (r AnalyticPredicate) Update(e event.UpdateEvent) bool {
	old, cast := e.ObjectOld.(*migapi.MigAnalytic)
	if !cast {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigAnalytic)
	if !cast {
		return true
	}
	changed := !reflect.DeepEqual(old.Spec, new.Spec)
	return changed
}

func (r AnalyticPredicate) Delete(e event.DeleteEvent) bool {
	return true
}
