package migcluster

import (
	"reflect"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ClusterPredicate struct {
	predicate.Funcs
	Namespace string
}

func (r ClusterPredicate) Create(e event.CreateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	cluster, cast := e.Object.(*migapi.MigCluster)
	if cast {
		r.mapRefs(cluster)
	}
	return true
}

func (r ClusterPredicate) Update(e event.UpdateEvent) bool {
	if r.Namespace != "" && r.Namespace != e.ObjectNew.GetNamespace() {
		return false
	}
	old, cast := e.ObjectOld.(*migapi.MigCluster)
	if !cast {
		return true
	}
	new, cast := e.ObjectNew.(*migapi.MigCluster)
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

func (r ClusterPredicate) Delete(e event.DeleteEvent) bool {
	if r.Namespace != "" && r.Namespace != e.Object.GetNamespace() {
		return false
	}
	cluster, cast := e.Object.(*migapi.MigCluster)
	if cast {
		r.unmapRefs(cluster)
	}
	return true
}

func (r ClusterPredicate) mapRefs(cluster *migapi.MigCluster) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(cluster),
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	// service account secret
	ref := cluster.Spec.ServiceAccountSecretRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}

func (r ClusterPredicate) unmapRefs(cluster *migapi.MigCluster) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(cluster),
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	// service account secret
	ref := cluster.Spec.ServiceAccountSecretRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}
