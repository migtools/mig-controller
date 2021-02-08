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

package directimagemigration

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
	migtrace "github.com/konveyor/mig-controller/pkg/tracing"
	"github.com/opentracing/opentracing-go"
)

func (r *ReconcileDirectImageMigration) initTracer(dim *migapi.DirectImageMigration) opentracing.Span {
	// Exit if tracing disabled
	if !settings.Settings.JaegerOpts.Enabled {
		return nil
	}
	// Set tracer on reconciler if it's not already present.
	// We will never close this, so the 'closer' is discarded.
	if r.tracer == nil {
		r.tracer, _ = migtrace.InitJaeger("DirectImageMigration")
	}

	// Get overall migration span
	var migrationSpan opentracing.Span
	ownerRefs := dim.GetOwnerReferences()
	if len(ownerRefs) > 0 {
		migrationUID := ownerRefs[0].UID
		migrationSpan = migtrace.GetSpanForMigrationUID(string(migrationUID))
		// Set up migrationSpan if doesn't exist yet
		if migrationSpan == nil {
			migrationSpan = r.tracer.StartSpan("migration-" + string(migrationUID))
			migtrace.SetSpanForMigrationUID(string(migrationUID), migrationSpan)
		}
	}

	// Get span for current reconcile
	var reconcileSpan opentracing.Span
	if migrationSpan != nil {
		reconcileSpan = r.tracer.StartSpan(
			"reconcile", opentracing.ChildOf(migrationSpan.Context()),
		)
	}

	return reconcileSpan
}
