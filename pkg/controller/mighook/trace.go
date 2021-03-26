/*
Copyright 2021 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mighook

import (
	"github.com/opentracing/opentracing-go"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
	migtrace "github.com/konveyor/mig-controller/pkg/tracing"
)

// Given a MigHook, return a reconcile-scoped Jaeger span.
func (r *ReconcileMigHook) initTracer(mighook *migapi.MigHook) opentracing.Span {
	// Exit if tracing disabled
	if !settings.Settings.JaegerOpts.Enabled {
		return nil
	}
	// Set tracer on reconciler if it's not already present.
	// We will never close this, so the 'closer' is discarded.
	if r.tracer == nil {
		r.tracer, _ = migtrace.InitJaeger("MigHook")
	}
	// Begin reconcile span
	reconcileSpan := r.tracer.StartSpan("mighook-reconcile-" + mighook.Name)

	return reconcileSpan
}
