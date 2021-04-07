package mighook

import (
	"context"
	"encoding/base64"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/opentracing/opentracing-go"
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
func (r ReconcileMigHook) validate(ctx context.Context, hook *migapi.MigHook) error {
	if opentracing.SpanFromContext(ctx) != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validate")
		defer span.Finish()
	}

	err := r.validateImage(ctx, hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateTargetCluster(ctx, hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validatePlaybookData(ctx, hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateCustom(ctx, hook)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (r ReconcileMigHook) validateImage(ctx context.Context, hook *migapi.MigHook) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateImage")
		defer span.Finish()
	}
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

func (r ReconcileMigHook) validateTargetCluster(ctx context.Context, hook *migapi.MigHook) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateTargetCluster")
		defer span.Finish()
	}

	if hook.Spec.TargetCluster != "source" && hook.Spec.TargetCluster != "destination" {
		hook.Status.SetCondition(migapi.Condition{
			Type:     InvalidTargetCluster,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "Only `source` and `destination` are accepted as values for spec.targetCluster.",
		})
	}
	return nil
}

func (r ReconcileMigHook) validatePlaybookData(ctx context.Context, hook *migapi.MigHook) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validatePlaybookData")
		defer span.Finish()
	}
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

func (r ReconcileMigHook) validateCustom(ctx context.Context, hook *migapi.MigHook) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateCustom")
		defer span.Finish()
	}
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
