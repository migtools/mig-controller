package mighook

import (
	"encoding/base64"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
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

// Validate the hook resource.
func (r ReconcileMigHook) validate(hook *migapi.MigHook) error {
	err := r.validateImage(hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateTargetCluster(hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validatePlaybookData(hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateCustom(hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (r ReconcileMigHook) validateImage(hook *migapi.MigHook) error {
	match := ReferenceRegexp.MatchString(hook.Spec.Image)

	if !match {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidImage,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The image name specified in spec.Image is invalid.",
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
			Message:  "Only 'source' and 'destination' are accepted as values for spec.targetCluster.",
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
			Message:  "Spec.Playbook should contain a base64 encoded playbook.",
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
			Message:  "An Ansible Playbook must not be specified when spec.custom is true.",
		})
	} else if !hook.Spec.Custom && hook.Spec.Playbook == "" {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidAnsibleHook,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "An Ansible Playbook must be specified when spec.custom is false.",
		})
	}
	return nil
}
