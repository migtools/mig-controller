package migstorage

import (
	"context"
	"fmt"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"strings"
)

// Types
const (
	InvalidBackupStorageProvider  = "InvalidBackupStorageProvider"
	InvalidVolumeSnapshotProvider = "InvalidVolumeSnapshotProvider"
)

// Reasons
const (
	Supported      = "Supported"
	NotSupported   = "NotSupported"
	InvalidSetting = "InvalidSetting"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage                         = "The storage is ready."
	InvalidBackupStorageProviderMessage  = "The `backupStorageProvider` must be: (aws|gcp|azure)."
	InvalidBackupSettingsMessage         = "The `backupStorageProvider` settings [%s] not valid."
	InvalidVolumeSnapshotProviderMessage = "The `volumeSnapshotProvider` must be: (aws|gcp|azure)."
	InvalidVolumeSnapshotSettingsMessage = "The `volumeSnapshotProvider` settings [%s] not valid."
)

// Validate the storage resource.
// Returns error and the total error conditions set.
func (r ReconcileMigStorage) validate(storage *migapi.MigStorage) (error, int) {
	totalSet := 0

	// Backup location provider.
	err, nSet := r.validateBackupStorage(storage)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Volume snapshot location provider.
	err, nSet = r.validateVolumeStorage(storage)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Ready
	storage.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes.
	err = r.Update(context.TODO(), storage)
	if err != nil {
		return err, 0
	}

	return nil, totalSet
}

func (r ReconcileMigStorage) validateBackupStorage(storage *migapi.MigStorage) (error, int) {
	provider := storage.Spec.BackupStorageProvider
	var err error
	nSet := 0

	switch provider {
	case "aws":
		err, nSet = r.validateAwsBackupStorage(storage)
	case "azure":
		err, nSet = r.validateAzureBackupStorage(storage)
	case "gcp":
	case "":
		err, nSet = nil, 0
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBackupStorageProvider,
			Status:  True,
			Reason:  NotSupported,
			Message: InvalidBackupStorageProviderMessage,
		})
		return nil, 1
	}

	if err == nil && nSet == 0 {
		storage.Status.DeleteCondition(InvalidBackupStorageProvider)
	}

	return err, nSet
}

func (r ReconcileMigStorage) validateAwsBackupStorage(storage *migapi.MigStorage) (error, int) {
	fields := make([]string, 0)
	cfg := storage.Spec.BackupStorageConfig

	// Validate settings.
	if cfg.AwsRegion == "" {
		fields = append(fields, "awsRegion")
	}
	if cfg.AwsPublicURL == "" {
		fields = append(fields, "awsPublicUrl")
	}
	if cfg.AwsBucketName == "" {
		fields = append(fields, "awsBucketName")
	}
	if cfg.AwsKmsKeyID == "" {
		fields = append(fields, "awsKmsKeyId")
	}
	if cfg.AwsSignatureVersion == "" {
		fields = append(fields, "awsSignatureVersion")
	}

	// Set condition.
	if len(fields) > 0 {
		message := fmt.Sprintf(InvalidBackupSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBackupStorageProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateAzureBackupStorage(storage *migapi.MigStorage) (error, int) {
	fields := make([]string, 0)
	cfg := storage.Spec.BackupStorageConfig

	// Validate settings.
	if cfg.AzureResourceGroup == "" {
		fields = append(fields, "azureResourceGroup")
	}
	if cfg.AzureStorageAccount == "" {
		fields = append(fields, "azureStorageAccount")
	}

	// Set condition.
	if len(fields) > 0 {
		message := fmt.Sprintf(InvalidBackupSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBackupStorageProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateVolumeStorage(storage *migapi.MigStorage) (error, int) {
	provider := storage.Spec.VolumeSnapshotProvider
	var err error
	nSet := 0

	switch provider {
	case "aws":
		err, nSet = r.validateAwsVolumeStorage(storage)
	case "azure":
		err, nSet = r.validateAzureVolumeStorage(storage)
	case "gcp":
	case "":
		err, nSet = nil, 0
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVolumeSnapshotProvider,
			Status:  True,
			Reason:  NotSupported,
			Message: InvalidVolumeSnapshotProviderMessage,
		})
		return nil, 1
	}

	if err == nil && nSet == 0 {
		storage.Status.DeleteCondition(InvalidVolumeSnapshotProvider)
	}

	return err, nSet
}

func (r ReconcileMigStorage) validateAwsVolumeStorage(storage *migapi.MigStorage) (error, int) {
	fields := make([]string, 0)
	cfg := storage.Spec.VolumeSnapshotConfig

	// Validate settings.
	if cfg.AwsRegion == "" {
		fields = append(fields, "awsRegion")
	}

	// Set condition.
	if len(fields) > 0 {
		message := fmt.Sprintf(InvalidVolumeSnapshotSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVolumeSnapshotProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateAzureVolumeStorage(storage *migapi.MigStorage) (error, int) {
	fields := make([]string, 0)
	cfg := storage.Spec.VolumeSnapshotConfig

	// Validate settings.
	if cfg.AzureResourceGroup == "" {
		fields = append(fields, "azureResourceGroup")
	}
	if cfg.AzureAPITimeout == "" {
		fields = append(fields, "azureAPITimeout")
	}

	// Set condition.
	if len(fields) > 0 {
		message := fmt.Sprintf(InvalidVolumeSnapshotSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVolumeSnapshotProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return nil, 1
	}

	return nil, 0
}
