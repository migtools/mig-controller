package migstorage

import (
	"context"
	"fmt"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"strings"
)

// Notes:
//   BSL = Backup Storage Location
//   VSL = Volume Snapshot Location

// Types
const (
	InvalidBSLProvider       = "InvalidBackupStorageProvider"
	InvalidBSLCredsSecretRef = "InvalidBackupStorageCredsSecretRef"
	InvalidBSLCredsSecret    = "InvalidBackupStorageCredsSecret"
	InvalidVSLProvider       = "InvalidVolumeSnapshotProvider"
	InvalidVSLCredsSecretRef = "InvalidVolumeSnapshotCredsSecretRef"
	InvalidVSLCredsSecret    = "InvalidVolumeSnapshotCredsSecret"
)

// Reasons
const (
	Supported      = "Supported"
	NotSupported   = "NotSupported"
	InvalidSetting = "InvalidSetting"
	NotSet         = "NotSet"
	NotFound       = "NotFound"
	KeyError       = "KeyError"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage                    = "The storage is ready."
	InvalidBSLProviderMessage       = "The `backupStorageProvider` must be: (aws|gcp|azure)."
	InvalidBSLSettingsMessage       = "The `backupStorageConfig` settings [%s] not valid."
	InvalidBSLCredsSecretRefMessage = "The `backupStorageConfig.credsSecretRef` must reference a `secret`."
	InvalidBSLCredsSecretMessage    = "The `backupStorageConfig.credsSecretRef` secret has invalid content."
	InvalidVSLProviderMessage       = "The `volumeSnapshotProvider` must be: (aws|gcp|azure)."
	InvalidVSLSettingsMessage       = "The `volumeSnapshotConfig` settings [%s] not valid."
	InvalidVSLCredsSecretRefMessage = "The `volumeSnapshotConfig.credsSecretRef` must reference a `secret`."
	InvalidVSLCredsSecretMessage    = "The `volumeSnapshotConfig.credsSecretRef` secret has invalid content."
)

// Validate the storage resource.
// Returns error and the total error conditions set.
func (r ReconcileMigStorage) validate(storage *migapi.MigStorage) (int, error) {
	totalSet := 0

	// Backup location provider.
	nSet, err := r.validateBSL(storage)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Volume snapshot location provider.
	nSet, err = r.validateVSL(storage)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Ready
	storage.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes.
	err = r.Update(context.TODO(), storage)
	if err != nil {
		return 0, err
	}

	return totalSet, err
}

func (r ReconcileMigStorage) validateBSL(storage *migapi.MigStorage) (int, error) {
	provider := storage.Spec.BackupStorageProvider
	var err error
	totalSet := 0
	nSet := 0

	switch provider {
	case "aws":
		nSet, err = r.validateAwsBSLSettings(storage)
	case "azure":
		nSet, err = r.validateAzureBSLSettings(storage)
	case "gcp":
	case "":
		nSet, err = 0, nil
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLProvider,
			Status:  True,
			Reason:  NotSupported,
			Message: InvalidBSLProviderMessage,
		})
		return 1, nil
	}

	totalSet += nSet
	if err == nil && nSet == 0 {
		storage.Status.DeleteCondition(InvalidBSLProvider)
	}

	nSet, err = r.validateBSLCredsSecret(storage)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Update
	err = r.Update(context.TODO(), storage)
	if err != nil {
		return 0, err
	}

	return totalSet, err
}

func (r ReconcileMigStorage) validateAwsBSLSettings(storage *migapi.MigStorage) (int, error) {
	fields := make([]string, 0)
	cfg := storage.Spec.BackupStorageConfig

	// Validate settings.
	if cfg.AwsRegion == "" {
		fields = append(fields, "awsRegion")
	}
	if cfg.AwsBucketName == "" {
		fields = append(fields, "awsBucketName")
	}
	v := cfg.AwsSignatureVersion
	if !(v == "" || v == "1" || v == "4") {
		fields = append(fields, "awsSignatureVersion")
	}

	// Set condition.
	if len(fields) > 0 {
		message := fmt.Sprintf(InvalidBSLSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return 1, nil
	}

	return 0, nil
}

func (r ReconcileMigStorage) validateAzureBSLSettings(storage *migapi.MigStorage) (int, error) {
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
		message := fmt.Sprintf(InvalidBSLSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return 1, nil
	}

	return 0, nil
}

func (r ReconcileMigStorage) validateBSLCredsSecret(storage *migapi.MigStorage) (int, error) {
	if storage.Spec.BackupStorageProvider == "" {
		storage.Status.DeleteCondition(InvalidBSLCredsSecretRef)
		storage.Status.DeleteCondition(InvalidBSLCredsSecret)
		return 0, nil
	}

	ref := storage.Spec.BackupStorageConfig.CredsSecretRef

	// NotSet
	if !migref.RefSet(ref) {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLCredsSecretRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidBSLCredsSecretRefMessage,
		})
		storage.Status.DeleteCondition(InvalidBSLCredsSecret)
		return 1, nil
	}

	secret, err := migapi.GetSecret(r, ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if secret == nil {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLCredsSecretRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidBSLCredsSecretRefMessage,
		})
		storage.Status.DeleteCondition(InvalidBSLCredsSecret)
		return 1, nil
	} else {
		storage.Status.DeleteCondition(InvalidBSLCredsSecretRef)
	}

	// secret content
	if !r.validCredsSecret(secret) {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLCredsSecret,
			Status:  True,
			Reason:  KeyError,
			Message: InvalidBSLCredsSecretMessage,
		})
		return 1, nil
	} else {
		storage.Status.DeleteCondition(InvalidBSLCredsSecret)
	}

	return 0, nil
}

func (r ReconcileMigStorage) validateVSL(storage *migapi.MigStorage) (int, error) {
	provider := storage.Spec.VolumeSnapshotProvider
	var err error
	totalSet := 0
	nSet := 0

	switch provider {
	case "aws":
		nSet, err = r.validateAwsVSLSettings(storage)
	case "azure":
		nSet, err = r.validateAzureVSLSettings(storage)
	case "gcp":
	case "":
		nSet, err = 0, nil
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLProvider,
			Status:  True,
			Reason:  NotSupported,
			Message: InvalidVSLProviderMessage,
		})
		return 1, nil
	}

	totalSet += nSet
	if err == nil && nSet == 0 {
		storage.Status.DeleteCondition(InvalidVSLProvider)
	}

	nSet, err = r.validateVSLCredsSecret(storage)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	return totalSet, err
}

func (r ReconcileMigStorage) validateAwsVSLSettings(storage *migapi.MigStorage) (int, error) {
	fields := make([]string, 0)
	cfg := storage.Spec.VolumeSnapshotConfig

	// Validate settings.
	if cfg.AwsRegion == "" {
		fields = append(fields, "awsRegion")
	}

	// Set condition.
	if len(fields) > 0 {
		message := fmt.Sprintf(InvalidVSLSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return 1, nil
	}

	return 0, nil
}

func (r ReconcileMigStorage) validateAzureVSLSettings(storage *migapi.MigStorage) (int, error) {
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
		message := fmt.Sprintf(InvalidVSLSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return 1, nil
	}

	return 0, nil
}

func (r ReconcileMigStorage) validateVSLCredsSecret(storage *migapi.MigStorage) (int, error) {
	if storage.Spec.VolumeSnapshotProvider == "" {
		storage.Status.DeleteCondition(InvalidVSLCredsSecretRef)
		storage.Status.DeleteCondition(InvalidVSLCredsSecret)
		return 0, nil
	}

	ref := storage.Spec.VolumeSnapshotConfig.CredsSecretRef

	// NotSet
	if !migref.RefSet(ref) {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLCredsSecretRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidVSLCredsSecretRefMessage,
		})
		storage.Status.DeleteCondition(InvalidVSLCredsSecret)
		return 1, nil
	}

	secret, err := migapi.GetSecret(r, ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if secret == nil {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLCredsSecretRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidVSLCredsSecretRefMessage,
		})
		storage.Status.DeleteCondition(InvalidVSLCredsSecret)
		return 1, nil
	} else {
		storage.Status.DeleteCondition(InvalidVSLCredsSecretRef)
	}

	// secret content
	if !r.validCredsSecret(secret) {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLCredsSecret,
			Status:  True,
			Reason:  KeyError,
			Message: InvalidVSLCredsSecretMessage,
		})
		return 1, nil
	} else {
		storage.Status.DeleteCondition(InvalidVSLCredsSecret)
	}

	return 0, nil
}

func (r ReconcileMigStorage) validCredsSecret(secret *kapi.Secret) bool {
	// TODO: waiting on secret layout.
	return true
}
