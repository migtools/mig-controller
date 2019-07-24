package migstorage

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
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

// Messages
const (
	ReadyMessage                   = "The storage is ready."
	InvalidBSProviderMessage       = "The `backupStorageProvider` must be: (aws|gcp|azure)."
	InvalidBSCredsSecretRefMessage = "The `backupStorageConfig.credsSecretRef` must reference a `secret`."
	InvalidBSFieldsMessage         = "The `backupStorageConfig` settings [] not valid."
	InvalidVSProviderMessage       = "The `volumeSnapshotProvider` must be: (aws|gcp|azure)."
	InvalidVSCredsSecretRefMessage = "The `volumeSnapshotConfig.credsSecretRef` must reference a `secret`."
	InvalidVSFieldsMessage         = "The `volumeSnapshotConfig` settings [] not valid."
	BSProviderTestFailedMessage    = "The Backup storage cloudprovider test failed []."
	VSProviderTestFailedMessage    = "The Volume Snapshot cloudprovider test failed []."
)

// Validate the storage resource.
func (r ReconcileMigStorage) validate(storage *migapi.MigStorage) error {
	err := r.validateBackupStorage(storage)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = r.validateVolumeSnapshotStorage(storage)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

func (r ReconcileMigStorage) validateBackupStorage(storage *migapi.MigStorage) error {
	settings := storage.Spec.BackupStorageConfig

	if storage.Spec.BackupStorageProvider == "" {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSProvider,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidBSProviderMessage,
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
			Message:  InvalidBSProviderMessage,
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
			Message:  InvalidBSCredsSecretRefMessage,
		})
	}

	// Secret
	secret, err := storage.GetBackupStorageCredSecret(r)
	if err != nil {
		log.Trace(err)
		return err
	}

	// NotFound
	if secret == nil {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSCredsSecretRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  InvalidBSCredsSecretRefMessage,
		})
	}

	// Fields
	fields := provider.Validate(secret)
	if len(fields) > 0 {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSFields,
			Status:   True,
			Reason:   NotSupported,
			Category: Critical,
			Message:  InvalidBSFieldsMessage,
			Items:    fields,
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
				Message:  BSProviderTestFailedMessage,
				Items:    []string{err.Error()},
			})
		}
	}

	return nil
}

func (r ReconcileMigStorage) validateVolumeSnapshotStorage(storage *migapi.MigStorage) error {
	settings := storage.Spec.VolumeSnapshotConfig

	// Provider
	provider := storage.GetVolumeSnapshotProvider()
	// Secret
	secret, err := storage.GetVolumeSnapshotCredSecret(r)
	if err != nil {
		log.Trace(err)
		return err
	}

	if storage.Spec.VolumeSnapshotProvider != "" {
		// Unknown provider.
		if provider == nil {
			storage.Status.SetCondition(migapi.Condition{
				Type:     InvalidVSProvider,
				Status:   True,
				Reason:   NotSupported,
				Category: Critical,
				Message:  InvalidVSProviderMessage,
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
				Message:  InvalidVSCredsSecretRefMessage,
			})
		}

		// NotFound
		if secret == nil {
			storage.Status.SetCondition(migapi.Condition{
				Type:     InvalidVSCredsSecretRef,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  InvalidVSCredsSecretRefMessage,
			})
		}

		// Fields
		fields := provider.Validate(secret)
		if len(fields) > 0 {
			storage.Status.SetCondition(migapi.Condition{
				Type:     InvalidVSFields,
				Status:   True,
				Reason:   NotSupported,
				Category: Critical,
				Message:  InvalidVSFieldsMessage,
				Items:    fields,
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
				Message:  VSProviderTestFailedMessage,
				Items:    []string{err.Error()},
			})
		}
	}

	return nil
}
