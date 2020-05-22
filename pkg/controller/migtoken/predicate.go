package migtoken

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type TokenPredicate struct {
	predicate.Funcs
}

func (r TokenPredicate) Create(e event.CreateEvent) bool {
	token, cast := e.Object.(*migapi.MigToken)
	if cast && token.InTenant() {
		r.mapRefs(token)
		return true
	}
	return false
}

func (r TokenPredicate) Update(e event.UpdateEvent) bool {
	old, cast := e.ObjectOld.(*migapi.MigToken)
	if !cast {
		return false
	}
	new, cast := e.ObjectNew.(*migapi.MigToken)
	if !cast {
		return false
	}
	if !old.InPrivileged() {
		return false
	}
	changed := !reflect.DeepEqual(old.Spec, new.Spec) ||
		!reflect.DeepEqual(old.DeletionTimestamp, new.DeletionTimestamp)
	if changed {
		r.unmapRefs(old)
		r.mapRefs(new)
	}
	return changed
}

func (r TokenPredicate) Delete(e event.DeleteEvent) bool {
	token, cast := e.Object.(*migapi.MigToken)
	if cast && token.InTenant() {
		r.unmapRefs(token)
		return true
	}
	return false
}

func (r TokenPredicate) Generic(e event.GenericEvent) bool {
	token, cast := e.Object.(*migapi.MigToken)
	if cast && token.InTenant() {
		r.mapRefs(token)
		return true
	}
	return false
}

func (r TokenPredicate) mapRefs(token *migapi.MigToken) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(token),
		Namespace: token.Namespace,
		Name:      token.Name,
	}

	// identity secret
	ref := token.Spec.SecretRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}

func (r TokenPredicate) unmapRefs(token *migapi.MigToken) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(token),
		Namespace: token.Namespace,
		Name:      token.Name,
	}

	// identity secret
	ref := token.Spec.SecretRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}
