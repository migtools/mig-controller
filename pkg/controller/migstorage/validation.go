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
func (r ReconcileMigStorage) validate(storage *migapi.MigStorage) error {
	storage.Status.BeginStagingConditions()

	// Backup location provider.
	err := r.validateBSL(storage)
	if err != nil {
		return err
	}

	// Volume snapshot location provider.
	err = r.validateVSL(storage)
	if err != nil {
		return err
	}

	// Ready
	storage.Status.SetReady(
		!storage.Status.HasBlockerCondition(),
		ReadyMessage)

	// Apply changes.
	storage.Status.EndStagingConditions()
	err = r.Update(context.TODO(), storage)
	if err != nil {
		return err
	}

	return nil
}

func (r ReconcileMigStorage) validateBSL(storage *migapi.MigStorage) error {
	provider := storage.Spec.BackupStorageProvider
	var err error

	switch provider {
	case "aws":
		err = r.validateAwsBSLSettings(storage)
	case "azure":
		err = r.validateAzureBSLSettings(storage)
	case "gcp":
	case "":
		err = nil
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSLProvider,
			Status:   True,
			Reason:   NotSupported,
			Category: migapi.Error,
			Message:  InvalidBSLProviderMessage,
		})
		return nil
	}

	err = r.validateBSLCredsSecret(storage)
	if err != nil {
		return err
	}

	// Update
	err = r.Update(context.TODO(), storage)
	if err != nil {
		return err
	}

	return err
}

func (r ReconcileMigStorage) validateAwsBSLSettings(storage *migapi.MigStorage) error {
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
			Type:     InvalidBSLProvider,
			Status:   True,
			Category: migapi.Error,
			Reason:   InvalidSetting,
			Message:  message,
		})
		return nil
	}

	return nil
}

func (r ReconcileMigStorage) validateAzureBSLSettings(storage *migapi.MigStorage) error {
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
			Type:     InvalidBSLProvider,
			Status:   True,
			Category: migapi.Error,
			Reason:   InvalidSetting,
			Message:  message,
		})
		return nil
	}

	return nil
}

func (r ReconcileMigStorage) validateBSLCredsSecret(storage *migapi.MigStorage) error {
	if storage.Spec.BackupStorageProvider == "" {
		return nil
	}

	ref := storage.Spec.BackupStorageConfig.CredsSecretRef

	// NotSet
	if !migref.RefSet(ref) {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSLCredsSecretRef,
			Status:   True,
			Reason:   NotSet,
			Category: migapi.Error,
			Message:  InvalidBSLCredsSecretRefMessage,
		})
		return nil
	}

	secret, err := migapi.GetSecret(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if secret == nil {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSLCredsSecretRef,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  InvalidBSLCredsSecretRefMessage,
		})
		return nil
	}

	// secret content
	if !r.validCredSecret(secret) {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidBSLCredsSecret,
			Status:   True,
			Reason:   KeyError,
			Category: migapi.Error,
			Message:  InvalidBSLCredsSecretMessage,
		})
		return nil
	}

	return nil
}

func (r ReconcileMigStorage) validateVSL(storage *migapi.MigStorage) error {
	provider := storage.Spec.VolumeSnapshotProvider
	var err error

	switch provider {
	case "aws":
		err = r.validateAwsVSLSettings(storage)
	case "azure":
		err = r.validateAzureVSLSettings(storage)
	case "gcp":
	case "":
		err = nil
	default:
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidVSLProvider,
			Status:   True,
			Reason:   NotSupported,
			Category: migapi.Error,
			Message:  InvalidVSLProviderMessage,
		})
		return nil
	}

	err = r.validateVSLCredsSecret(storage)
	if err != nil {
		return err
	}

	return err
}

func (r ReconcileMigStorage) validateAwsVSLSettings(storage *migapi.MigStorage) error {
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
			Type:     InvalidVSLProvider,
			Status:   True,
			Reason:   InvalidSetting,
			Category: migapi.Error,
			Message:  message,
		})
		return nil
	}

	return nil
}

func (r ReconcileMigStorage) validateAzureVSLSettings(storage *migapi.MigStorage) error {
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
			Type:     InvalidVSLProvider,
			Status:   True,
			Reason:   InvalidSetting,
			Category: migapi.Error,
			Message:  message,
		})
		return nil
	}

	return nil
}

func (r ReconcileMigStorage) validateVSLCredsSecret(storage *migapi.MigStorage) error {
	if storage.Spec.VolumeSnapshotProvider == "" {
		return nil
	}

	ref := storage.Spec.VolumeSnapshotConfig.CredsSecretRef

	// NotSet
	if !migref.RefSet(ref) {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidVSLCredsSecretRef,
			Status:   True,
			Reason:   NotSet,
			Category: migapi.Error,
			Message:  InvalidVSLCredsSecretRefMessage,
		})
		return nil
	}

	secret, err := migapi.GetSecret(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if secret == nil {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidVSLCredsSecretRef,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  InvalidVSLCredsSecretRefMessage,
		})
		return nil
	}

	// secret content
	if !r.validCredSecret(secret) {
		storage.Status.SetCondition(migapi.Condition{
			Type:     InvalidVSLCredsSecret,
			Status:   True,
			Reason:   KeyError,
			Category: migapi.Error,
			Message:  InvalidVSLCredsSecretMessage,
		})
		return nil
	}

	return nil
}

func (r ReconcileMigStorage) validCredSecret(secret *kapi.Secret) bool {
	// TODO: waiting on secret layout.
	return true
}
