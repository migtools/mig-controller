package remotewatcher

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func hasCorrelationLabel(labels map[string]string) bool {
	for label := range labels {
		_, found := migapi.KnownLabels[label]
		if found {
			return true
		}
	}
	return false
}

// Secret
type SecretPredicate struct {
	predicate.Funcs
}

// Watched resource has been created.
func (r SecretPredicate) Create(e event.CreateEvent) bool {
	secret, cast := e.Object.(*kapi.Secret)
	if cast {
		return hasCorrelationLabel(secret.Labels)
	}
	return false
}

// Watched resource has been updated.
func (r SecretPredicate) Update(e event.UpdateEvent) bool {
	secret, cast := e.ObjectOld.(*kapi.Secret)
	if cast {
		return hasCorrelationLabel(secret.Labels)
	}
	return false
}

// Watched resource has been deleted.
func (r SecretPredicate) Delete(e event.DeleteEvent) bool {
	secret, cast := e.Object.(*kapi.Secret)
	if cast {
		return hasCorrelationLabel(secret.Labels)
	}
	return false
}

// Backup
type BackupPredicate struct {
	predicate.Funcs
}

// Watched resource has been created.
func (r BackupPredicate) Create(e event.CreateEvent) bool {
	backup, cast := e.Object.(*velero.Backup)
	if cast {
		return hasCorrelationLabel(backup.Labels)
	}
	return false
}

// Watched resource has been updated.
func (r BackupPredicate) Update(e event.UpdateEvent) bool {
	backup, cast := e.ObjectOld.(*velero.Backup)
	if cast {
		return hasCorrelationLabel(backup.Labels)
	}
	return false
}

// Watched resource has been deleted.
func (r BackupPredicate) Delete(e event.DeleteEvent) bool {
	backup, cast := e.Object.(*velero.Backup)
	if cast {
		return hasCorrelationLabel(backup.Labels)
	}
	return false
}

// Restore
type RestorePredicate struct {
	predicate.Funcs
}

// Watched resource has been created.
func (r RestorePredicate) Create(e event.CreateEvent) bool {
	restore, cast := e.Object.(*velero.Restore)
	if cast {
		return hasCorrelationLabel(restore.Labels)
	}
	return false
}

// Watched resource has been updated.
func (r RestorePredicate) Update(e event.UpdateEvent) bool {
	restore, cast := e.ObjectOld.(*velero.Restore)
	if cast {
		return hasCorrelationLabel(restore.Labels)
	}
	return false
}

// Watched resource has been deleted.
func (r RestorePredicate) Delete(e event.DeleteEvent) bool {
	restore, cast := e.Object.(*velero.Restore)
	if cast {
		return hasCorrelationLabel(restore.Labels)
	}
	return false
}

// BSL
type BSLPredicate struct {
	predicate.Funcs
}

// Watched resource has been created.
func (r BSLPredicate) Create(e event.CreateEvent) bool {
	bsl, cast := e.Object.(*velero.BackupStorageLocation)
	if cast {
		return hasCorrelationLabel(bsl.Labels)
	}
	return false
}

// Watched resource has been updated.
func (r BSLPredicate) Update(e event.UpdateEvent) bool {
	bsl, cast := e.ObjectOld.(*velero.BackupStorageLocation)
	if cast {
		return hasCorrelationLabel(bsl.Labels)
	}
	return false
}

// Watched resource has been deleted.
func (r BSLPredicate) Delete(e event.DeleteEvent) bool {
	bsl, cast := e.Object.(*velero.BackupStorageLocation)
	if cast {
		return hasCorrelationLabel(bsl.Labels)
	}
	return false
}

// VSL
type VSLPredicate struct {
	predicate.Funcs
}

// Watched resource has been created.
func (r VSLPredicate) Create(e event.CreateEvent) bool {
	vsl, cast := e.Object.(*velero.VolumeSnapshotLocation)
	if cast {
		return hasCorrelationLabel(vsl.Labels)
	}
	return false
}

// Watched resource has been updated.
func (r VSLPredicate) Update(e event.UpdateEvent) bool {
	vsl, cast := e.ObjectOld.(*velero.VolumeSnapshotLocation)
	if cast {
		return hasCorrelationLabel(vsl.Labels)
	}
	return false
}

// Watched resource has been deleted.
func (r VSLPredicate) Delete(e event.DeleteEvent) bool {
	vsl, cast := e.Object.(*velero.VolumeSnapshotLocation)
	if cast {
		return hasCorrelationLabel(vsl.Labels)
	}
	return false
}
