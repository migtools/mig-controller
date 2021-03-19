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

package directvolumemigrationprogress

import (
	"github.com/konveyor/mig-controller/pkg/errorutil"
	"github.com/konveyor/mig-controller/pkg/settings"
	migtrace "github.com/konveyor/mig-controller/pkg/tracing"
	"github.com/opentracing/opentracing-go"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (r *ReconcileDirectVolumeMigrationProgress) initTracer(dvmp migapi.DirectVolumeMigrationProgress) (opentracing.Span, error) {
	// Exit if tracing disabled
	if !settings.Settings.JaegerOpts.Enabled {
		return nil, nil
	}

	// Set tracer on reconciler if it's not already present.
	// We will never close this, so the 'closer' is discarded.
	if r.tracer == nil {
		r.tracer, _ = migtrace.InitJaeger("DirectVolumeMigrationProgress")
	}

	// Go from dvmp -> dvm -> migration, use migration UID to get span
	dvm, err := dvmp.GetDVMforDVMP(r)
	if err != nil {
		if errors.IsNotFound(errorutil.Unwrap(err)) {
			return nil, nil
		}
		return nil, liberr.Wrap(err)
	}
	if dvm == nil {
		return nil, nil
	}
	// dvm -> migration
	migration, err := dvm.GetMigrationForDVM(r)
	if err != nil {
		if errors.IsNotFound(errorutil.Unwrap(err)) {
			return nil, nil
		}
		return nil, liberr.Wrap(err)
	}
	if migration == nil {
		return nil, nil
	}
	// Get overall migration span
	migrationUID := string(migration.GetUID())
	migrationSpan := migtrace.GetSpanForMigrationUID(migrationUID)
	if migrationSpan == nil {
		migrationSpan = r.tracer.StartSpan("migration-" + migrationUID)
		migtrace.SetSpanForMigrationUID(migrationUID, migrationSpan)
	}

	// Get span for current reconcile
	var reconcileSpan opentracing.Span
	if migrationSpan != nil {
		reconcileSpan = r.tracer.StartSpan(
			"reconcile"+dvmp.Name, opentracing.ChildOf(migrationSpan.Context()),
		)
	}

	return reconcileSpan, nil
}
