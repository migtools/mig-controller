package migstorage

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/common"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type StoragePredicate struct {
	predicate.Funcs
}

func (r StoragePredicate) Create(e event.CreateEvent) bool {
	if !common.IsInSandboxNamespace(e.Meta.GetNamespace()) {
		return false
	}
	storage, cast := e.Object.(*migapi.MigStorage)
	if cast {
		r.mapRefs(storage)
	}
	return true
}

func (r StoragePredicate) Update(e event.UpdateEvent) bool {
	if !common.IsInSandboxNamespace(e.MetaNew.GetNamespace()) {
		return false
	}
	old, cast := e.ObjectOld.(*migapi.MigStorage)
	if !cast {
		return true
	}
	new, cast := e.ObjectNew.(*migapi.MigStorage)
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

func (r StoragePredicate) Delete(e event.DeleteEvent) bool {
	if !common.IsInSandboxNamespace(e.Meta.GetNamespace()) {
		return false
	}
	storage, cast := e.Object.(*migapi.MigStorage)
	if cast {
		r.unmapRefs(storage)
	}
	return true
}

func (r StoragePredicate) Generic(e event.GenericEvent) bool {
	return common.IsInSandboxNamespace(e.Meta.GetNamespace())
}

func (r StoragePredicate) mapRefs(storage *migapi.MigStorage) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(storage),
		Namespace: storage.Namespace,
		Name:      storage.Name,
	}

	// BSL cred secret
	ref := storage.Spec.BackupStorageConfig.CredsSecretRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
	// VSL cred secret
	ref = storage.Spec.VolumeSnapshotConfig.CredsSecretRef
	if migref.RefSet(ref) {
		refMap.Add(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}

func (r StoragePredicate) unmapRefs(storage *migapi.MigStorage) {
	refMap := migref.GetMap()

	refOwner := migref.RefOwner{
		Kind:      migref.ToKind(storage),
		Namespace: storage.Namespace,
		Name:      storage.Name,
	}

	// BSL cred secret
	ref := storage.Spec.BackupStorageConfig.CredsSecretRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
	// VSL cred secret
	ref = storage.Spec.VolumeSnapshotConfig.CredsSecretRef
	if migref.RefSet(ref) {
		refMap.Delete(refOwner, migref.RefTarget{
			Kind:      migref.ToKind(kapi.Secret{}),
			Namespace: ref.Namespace,
			Name:      ref.Name,
		})
	}
}
