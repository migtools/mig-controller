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
	InvalidVSLCredsSecretRefMessage = "The `backupStorageConfig.credsSecretRef` must reference a `secret`."
	InvalidVSLCredsSecretMessage    = "The `volumeSnapshotConfig.credsSecretRef` secret has invalid content."
)

// Validate the storage resource.
// Returns error and the total error conditions set.
func (r ReconcileMigStorage) validate(storage *migapi.MigStorage) (error, int) {
	totalSet := 0

	// Backup location provider.
	err, nSet := r.validateBSL(storage)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Volume snapshot location provider.
	err, nSet = r.validateVSL(storage)
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

func (r ReconcileMigStorage) validateBSL(storage *migapi.MigStorage) (error, int) {
	provider := storage.Spec.BackupStorageProvider
	var err error
	totalSet := 0
	nSet := 0

	switch provider {
	case "aws":
		err, nSet = r.validateAwsBSLSettings(storage)
	case "azure":
		err, nSet = r.validateAzureBSLSettings(storage)
	case "gcp":
	case "":
		err, nSet = nil, 0
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLProvider,
			Status:  True,
			Reason:  NotSupported,
			Message: InvalidBSLProviderMessage,
		})
		return nil, 1
	}

	totalSet += nSet
	if err == nil && nSet == 0 {
		storage.Status.DeleteCondition(InvalidBSLProvider)
	}

	err, nSet = r.validateBSLCredsSecret(storage)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	return err, totalSet
}

func (r ReconcileMigStorage) validateAwsBSLSettings(storage *migapi.MigStorage) (error, int) {
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
		message := fmt.Sprintf(InvalidBSLSettingsMessage, strings.Join(fields, ", "))
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLProvider,
			Status:  True,
			Reason:  InvalidSetting,
			Message: message,
		})
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateAzureBSLSettings(storage *migapi.MigStorage) (error, int) {
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
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateBSLCredsSecret(storage *migapi.MigStorage) (error, int) {
	if storage.Spec.BackupStorageProvider == "" {
		storage.Status.DeleteCondition(InvalidBSLCredsSecretRef)
		storage.Status.DeleteCondition(InvalidBSLCredsSecret)
		return nil, 0
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
		return nil, 1
	}

	err, secret := migapi.GetSecret(r, ref)
	if err != nil {
		return err, 0
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
		return nil, 1
	} else {
		storage.Status.DeleteCondition(InvalidBSLCredsSecret)
	}

	// secret content
	if !r.validCredsSecret(secret) {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidBSLCredsSecret,
			Status:  True,
			Reason:  KeyError,
			Message: InvalidBSLCredsSecretMessage,
		})
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateVSL(storage *migapi.MigStorage) (error, int) {
	provider := storage.Spec.VolumeSnapshotProvider
	var err error
	totalSet := 0
	nSet := 0

	switch provider {
	case "aws":
		err, nSet = r.validateAwsVSLSettings(storage)
	case "azure":
		err, nSet = r.validateAzureVSLSettings(storage)
	case "gcp":
	case "":
		err, nSet = nil, 0
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLProvider,
			Status:  True,
			Reason:  NotSupported,
			Message: InvalidVSLProviderMessage,
		})
		return nil, 1
	}

	totalSet += nSet
	if err == nil && nSet == 0 {
		storage.Status.DeleteCondition(InvalidVSLProvider)
	}

	err, nSet = r.validateVSLCredsSecret(storage)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	return err, totalSet
}

func (r ReconcileMigStorage) validateAwsVSLSettings(storage *migapi.MigStorage) (error, int) {
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
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateAzureVSLSettings(storage *migapi.MigStorage) (error, int) {
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
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validateVSLCredsSecret(storage *migapi.MigStorage) (error, int) {
	if storage.Spec.VolumeSnapshotProvider == "" {
		storage.Status.DeleteCondition(InvalidVSLCredsSecretRef)
		storage.Status.DeleteCondition(InvalidVSLCredsSecret)
		return nil, 0
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
		return nil, 1
	}

	err, secret := migapi.GetSecret(r, ref)
	if err != nil {
		return err, 0
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
		return nil, 1
	} else {
		storage.Status.DeleteCondition(InvalidVSLCredsSecret)
	}

	// secret content
	if !r.validCredsSecret(secret) {
		storage.Status.SetCondition(migapi.Condition{
			Type:    InvalidVSLCredsSecret,
			Status:  True,
			Reason:  KeyError,
			Message: InvalidVSLCredsSecretMessage,
		})
		return nil, 1
	}

	return nil, 0
}

func (r ReconcileMigStorage) validCredsSecret(secret *kapi.Secret) bool {
	return true
}
