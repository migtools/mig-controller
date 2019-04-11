package migassetcollection

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type AssetCollectionPredicate struct {
	predicate.Funcs
}

func (r AssetCollectionPredicate) Create(e event.CreateEvent) bool {
	assetCollection, cast := e.Object.(*migapi.MigAssetCollection)
	if cast {
		r.mapRefs(assetCollection)
	}
	return true
}

func (r AssetCollectionPredicate) Update(e event.UpdateEvent) bool {
	old, cast := e.ObjectOld.(*migapi.MigAssetCollection)
	if !cast {
		return true
	}
	new, cast := e.ObjectNew.(*migapi.MigAssetCollection)
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

func (r AssetCollectionPredicate) Delete(e event.DeleteEvent) bool {
	assetCollection, cast := e.Object.(*migapi.MigAssetCollection)
	if cast {
		r.unmapRefs(assetCollection)
	}
	return true
}

func (r AssetCollectionPredicate) mapRefs(assetCollection *migapi.MigAssetCollection) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(assetCollection),
		Namespace: assetCollection.Namespace,
		Name:      assetCollection.Name,
	}

	// namespaces
	for _, name := range assetCollection.Spec.Namespaces {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Namespace{}),
			Namespace: "",
			Name:      name,
		})
	}

}

func (r AssetCollectionPredicate) unmapRefs(assetCollection *migapi.MigAssetCollection) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(assetCollection),
		Namespace: assetCollection.Namespace,
		Name:      assetCollection.Name,
	}

	// namespaces
	for _, name := range assetCollection.Spec.Namespaces {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Namespace{}),
			Namespace: "",
			Name:      name,
		})
	}
}
