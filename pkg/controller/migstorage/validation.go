package migstorage

import (
	"context"
	"fmt"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"strings"
)

// Types
const (
	// Type
	Ready                         = "Ready"
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
	True  = "True"
	False = "False"
)

// Messages
const (
	ReadyMessage                        = "The storage is ready."
	InvalidBackupStorageProviderMessage = "The `backupStorageProvider` must be: (aws|gcp|azure)."
	InvalidSettingsMessage              = "The `backupStorageProvider` settings [%s] not valid."
)

// Validate the storage resource.
// Returns error and the total error conditions set.
func (r ReconcileMigStorage) validate(storage *migapi.MigStorage) (error, int) {
	totalSet := 0

	// Backup location provider.
	err, nSet := r.validateStorageProvider(storage)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Apply changes.
	err = r.Update(context.TODO(), storage)
	if err != nil {
		return err, 0
	}

	return nil, totalSet
}

func (r ReconcileMigStorage) validateStorageProvider(storage *migapi.MigStorage) (error, int) {
	provider := storage.Spec.BackupStorageProvider
	var err error
	nSet := 0

	switch provider {
	case "aws":
		err, nSet = r.validateAwsProvider(storage)
	case "gcp":
		err, nSet = r.validateGcpProvider(storage)
	case "azure":
		err, nSet = r.validateAzureProvider(storage)
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

func (r ReconcileMigStorage) validateAwsProvider(storage *migapi.MigStorage) (error, int) {
	fields := make([]string, 0)
	cfg := storage.Spec.BackupStorageConfig

	// Validate settings.
	if cfg.AwsRegion == "" {
		fields = append(fields, "awsRegion")
	}
	if cfg.AwsPublicURL == "" {
		fields = append(fields, "awsPublicURL")
	}
	if cfg.AwsBucketName == "" {
		fields = append(fields, "awsBucketName")
	}
	if cfg.AwsKmsKeyID == "" {
		fields = append(fields, "awsKmsKeyID")
	}
	if cfg.AwsSignatureVersion == "" {
		fields = append(fields, "awsSignatureVersion")
	}

	// Set condition.
	if len(fields) > 0 {
		message := fmt.Sprintf(InvalidSettingsMessage, strings.Join(fields, ", "))
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

func (r ReconcileMigStorage) validateGcpProvider(storage *migapi.MigStorage) (error, int) {
	return nil, 0
}

func (r ReconcileMigStorage) validateAzureProvider(storage *migapi.MigStorage) (error, int) {
	return nil, 0
}
