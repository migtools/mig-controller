package mighook

import (
	"encoding/base64"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/reference"
)

// Types
const (
	InvalidTargetCluster = "InvalidTarget"
	InvalidImage         = "InvalidImage"
	InvalidPlaybookData  = "InvalidPlaybookData"
	InvalidAnsibleHook   = "InvalidAnsibleHook"
	InvalidCustomHook    = "InvalidCustomHook"
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
	ReadyMessage                = "The hook is ready."
	InvalidTargetClusterMessage = "Only 'source' and 'destination' are accepted as values for spec.targetCluster."
	InvalidImageMessage         = "The image name specified in spec.Image is an invalid."
	InvalidPlaybookDataMessage  = "Spec.Playbook should contain a base64 encoded playbook."
	InvalidAnsibleHookMessage   = "An Ansible Playbook must be specified spec.custom is false"
	InvalidCustomHookMessage    = "An Ansible Playbook must not be specified spec.custom is true"
)

// Validate the hook resource.
func (r ReconcileMigHook) validate(hook *migapi.MigHook) error {
	err := r.validateImage(hook)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = r.validateTargetCluster(hook)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = r.validatePlaybookData(hook)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = r.validateCustom(hook)
	if err != nil {
		log.Trace(err)
		return err
	}
	return nil
}

func (r ReconcileMigHook) validateImage(hook *migapi.MigHook) error {
	match := reference.ReferenceRegexp.MatchString(hook.Spec.Image)

	if !match {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidImage,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidImageMessage,
		})
	}
	return nil
}

func (r ReconcileMigHook) validateTargetCluster(hook *migapi.MigHook) error {
	if hook.Spec.TargetCluster != "source" && hook.Spec.TargetCluster != "destination" {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidTargetCluster,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidTargetClusterMessage,
		})
	}
	return nil
}

func (r ReconcileMigHook) validatePlaybookData(hook *migapi.MigHook) error {
	if _, err := base64.StdEncoding.DecodeString(hook.Spec.Playbook); err != nil {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlaybookData,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidPlaybookDataMessage,
		})
	}
	return nil
}

func (r ReconcileMigHook) validateCustom(hook *migapi.MigHook) error {
	if hook.Spec.Custom && hook.Spec.Playbook != "" {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidCustomHook,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidCustomHookMessage,
		})
	} else if !hook.Spec.Custom && hook.Spec.Playbook == "" {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidAnsibleHook,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidAnsibleHookMessage,
		})
	}
	return nil
}
