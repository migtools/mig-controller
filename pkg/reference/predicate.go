package reference

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type MigrationNamespacePredicate struct {
	predicate.Funcs
	Namespace string
}

func (m MigrationNamespacePredicate) Create(evt event.CreateEvent) bool {
	if evt.Object.GetNamespace() == m.Namespace {
		return true
	}
	return false
}

func (m MigrationNamespacePredicate) Update(evt event.UpdateEvent) bool {
	if evt.ObjectNew.GetNamespace() == m.Namespace {
		return true
	}
	return false
}

func (m MigrationNamespacePredicate) Delete(evt event.DeleteEvent) bool {
	if evt.Object.GetNamespace() == m.Namespace {
		return true
	}
	return false
}
