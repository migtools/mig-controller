package migstorage

import (
	"context"
	"fmt"
	"path"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
)

// Notes:
//   BSL = Backup Storage Location
//   VSL = Volume Snapshot Location

// Types
const (
	InvalidBSProvider       = "InvalidBackupStorageProvider"
	InvalidBSCredsSecretRef = "InvalidBackupStorageCredsSecretRef"
	InvalidBSFields         = "InvalidBackupStorageSettings"
	InvalidVSProvider       = "InvalidVolumeSnapshotProvider"
	InvalidVSCredsSecretRef = "InvalidVolumeSnapshotCredsSecretRef"
	InvalidVSFields         = "InvalidVolumeSnapshotSettings"
	BSProviderTestFailed    = "BackupStorageProviderTestFailed"
	VSProviderTestFailed    = "VolumeSnapshotProviderTestFailed"
)

// Categories
const (
	Critical = migapi.Critical
)

// Reasons
const (
	Supported    = "Supported"
	NotSupported = "NotSupported"
	NotSet       = "NotSet"
	NotFound     = "NotFound"
	KeyError     = "KeyError"
	TestFailed   = "TestFailed"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Validate the storage resource.
func (r ReconcileMigStorage) validate(ctx context.Context, storage *migapi.MigStorage) error {
	if opentracing.SpanFromContext(ctx) != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validate")
		defer span.Finish()
	}
	err := r.validateBackupStorage(ctx, storage)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateVolumeSnapshotStorage(ctx, storage)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

func (r ReconcileMigStorage) validateBackupStorage(ctx context.Context, storage *migapi.MigStorage) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateBackupStorage")
		defer span.Finish()
	}

	settings := storage.Spec.BackupStorageConfig

	if storage.Spec.BackupStorageProvider == "" {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSProvider,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `spec.BackupStorageProvider` must be: (aws|gcp|azure).",
		})
		return nil
	}

	provider := storage.GetBackupStorageProvider()

	// Unknown provider.
	if provider == nil {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSProvider,
			Status:   True,
			Reason:   NotSupported,
			Category: Critical,
			Message: fmt.Sprintf("The `spec.BackupStorageProvider` must be: (aws|gcp|azure),"+
				" provider %s", storage.Spec.BackupStorageProvider),
		})
		return nil
	}

	// NotSet
	if !migref.RefSet(settings.CredsSecretRef) {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSCredsSecretRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `backupStorageConfig.credsSecretRef` must reference a valid `secret`.",
		})
		return nil
	}

	// Secret
	secret, err := storage.GetBackupStorageCredSecret(r)
	if err != nil {
		return liberr.Wrap(err)
	}

	// NotFound
	if secret == nil {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSCredsSecretRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message: fmt.Sprintf("The `backupStorageConfig.credsSecretRef` must reference a valid `secret`, "+
				"subject: %s.", path.Join(storage.Spec.BackupStorageConfig.CredsSecretRef.Namespace,
				storage.Spec.BackupStorageConfig.CredsSecretRef.Name)),
		})
		return nil
	}

	// Fields
	fields := provider.Validate(secret)
	if len(fields) > 0 {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSFields,
			Status:   True,
			Reason:   NotSupported,
			Category: Critical,
			Message: fmt.Sprintf("The `backupStorageConfig.credsSecretRef` must reference a valid `secret`,"+
				" subject: %s, [].", path.Join(storage.Spec.BackupStorageConfig.CredsSecretRef.Namespace,
				storage.Spec.BackupStorageConfig.CredsSecretRef.Name)),
			Items: fields,
		})
		return nil
	}

	// Test provider.
	if !storage.Status.HasBlockerCondition() {
		err = provider.Test(secret)
		if err != nil {
			storage.Status.SetCondition(migapi.Condition{
				Type:     BSProviderTestFailed,
				Status:   True,
				Reason:   TestFailed,
				Category: Critical,
				Message: fmt.Sprintf("The `backupStorageConfig` settings [] not provided in "+
					"secret %s not valid.", path.Join(secret.Namespace, secret.Name)),
				Items: []string{err.Error()},
			})
		}
	}

	return nil
}

func (r ReconcileMigStorage) validateVolumeSnapshotStorage(ctx context.Context, storage *migapi.MigStorage) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateVolumeSnapshotStorage")
		defer span.Finish()
	}

	settings := storage.Spec.VolumeSnapshotConfig

	// Provider
	provider := storage.GetVolumeSnapshotProvider()
	// Secret
	secret, err := storage.GetVolumeSnapshotCredSecret(r)
	if err != nil {
		return liberr.Wrap(err)
	}

	if storage.Spec.VolumeSnapshotProvider != "" {
		// Unknown provider.
		if provider == nil {
			storage.Status.SetCondition(migapi.Condition{
				Type:     InvalidVSProvider,
				Status:   True,
				Reason:   NotSupported,
				Category: Critical,
				Message: fmt.Sprintf("The `volumeSnapshotProvider` must be: (aws|gcp|azure).,"+
					" provider: %s", storage.Spec.VolumeSnapshotProvider),
			})
			return nil
		}

		// NotSet
		if !migref.RefSet(settings.CredsSecretRef) {
			storage.Status.SetCondition(migapi.Condition{
				Type:     InvalidVSCredsSecretRef,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "The `volumeSnapshotConfig.credsSecretRef` must reference a valid `secret`.",
			})
			return nil
		}

		// NotFound
		if secret == nil {
			storage.Status.SetCondition(migapi.Condition{
				Type:     InvalidVSCredsSecretRef,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message: fmt.Sprintf("The `volumeSnapshotConfig.credsSecretRef` must reference a `secret`."+
					" subject: %s", path.Join(storage.Spec.VolumeSnapshotConfig.CredsSecretRef.Namespace,
					storage.Spec.VolumeSnapshotConfig.CredsSecretRef.Namespace)),
			})
			return nil
		}

		// Fields
		fields := provider.Validate(secret)
		if len(fields) > 0 {
			storage.Status.SetCondition(migapi.Condition{
				Type:     InvalidVSFields,
				Status:   True,
				Reason:   NotSupported,
				Category: Critical,
				Message: fmt.Sprintf("The `volumeSnapshotConfig` settings [] in secret %s not valid.",
					path.Join(secret.Namespace, secret.Name)),
				Items: fields,
			})
			return nil
		}
	}

	// Test provider.
	if !storage.Status.HasBlockerCondition() {
		err = provider.Test(secret)
		if err != nil {
			storage.Status.SetCondition(migapi.Condition{
				Type:     VSProviderTestFailed,
				Status:   True,
				Reason:   TestFailed,
				Category: Critical,
				Message:  "The Volume Snapshot cloudprovider test failed [].",
				Items:    []string{err.Error()},
			})
		}
	}

	return nil
}
