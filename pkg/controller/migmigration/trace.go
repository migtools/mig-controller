/*
Copyright 2020 Red Hat Inc.

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

package migmigration

import (
	"github.com/opentracing/opentracing-go"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
	migtrace "github.com/konveyor/mig-controller/pkg/tracing"
)

// Given a migration, return a migration-scoped and reconcile-scoped Jaeger spans.
func (r *ReconcileMigMigration) initTracer(migration *migapi.MigMigration) (opentracing.Span, opentracing.Span) {
	// Exit if tracing disabled
	if !settings.Settings.JaegerOpts.Enabled {
		return nil, nil
	}
	// Exit if migration is finished
	if migration.Status.Phase == Completed {
		return nil, nil
	}

	// Set tracer on reconciler if it's not already present.
	// We will never close this, so the 'closer' is discarded.
	if r.tracer == nil {
		r.tracer, _ = migtrace.InitJaeger("MigMigration")
	}
	// Get migration span, add to global map if does not yet exist.
	migrationUID := string(migration.GetUID())
	migrationSpan := migtrace.GetSpanForMigrationUID(migrationUID)
	if migrationSpan == nil {
		migrationSpan = r.tracer.StartSpan("migration-" + migrationUID)
		migtrace.SetSpanForMigrationUID(migrationUID, migrationSpan)
	}
	// Begin reconcile span
	reconcileSpan := r.tracer.StartSpan(
		"migration-reconcile-"+migration.Name, opentracing.ChildOf(migrationSpan.Context()),
	)

	return migrationSpan, reconcileSpan
}
