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

package tracing

import (
	"fmt"
	"io"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"
)

var msmInstance MigrationSpanMap
var createMsmMapOnce sync.Once

// MigrationSpanMap provides a map between MigMigration UID and associated Jaeger span.
// This is required so that all controllers can attach child spans to the correct
// parent migmigration span for unified tracing of work done during migrations.
type MigrationSpanMap struct {
	mutex              sync.RWMutex
	migrationUIDToSpan map[string]opentracing.Span
}

// InitJaeger returns an instance of Jaeger Tracer that samples 100% of traces and logs all spans to stdout.
func InitJaeger(service string) (opentracing.Tracer, io.Closer) {
	cfg := &config.Configuration{
		ServiceName: service,
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
	}
	tracer, closer, err := cfg.NewTracer(config.Logger(jaeger.StdLogger))
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	return tracer, closer
}

// SetSpanForMigrationUID sets the parent jaeger span for a migration
func SetSpanForMigrationUID(migrationUID string, span opentracing.Span) {
	// Init map if needed
	createMsmMapOnce.Do(func() {
		msmInstance = MigrationSpanMap{}
		msmInstance.migrationUIDToSpan = make(map[string]opentracing.Span)
	})
	msmInstance.mutex.RLock()
	defer msmInstance.mutex.RUnlock()

	msmInstance.migrationUIDToSpan[migrationUID] = span
	return
}

// RemoveSpanForMigrationUID removes a span from the span map once migration is complete.
func RemoveSpanForMigrationUID(migrationUID string) {
	// Init map if needed
	createMsmMapOnce.Do(func() {
		msmInstance = MigrationSpanMap{}
		msmInstance.migrationUIDToSpan = make(map[string]opentracing.Span)
	})
	msmInstance.mutex.RLock()
	defer msmInstance.mutex.RUnlock()

	delete(msmInstance.migrationUIDToSpan, migrationUID)
	return
}

// GetSpanForMigrationUID returns the parent jaeger span for a migration
func GetSpanForMigrationUID(migrationUID string) opentracing.Span {
	// Init map if needed
	createMsmMapOnce.Do(func() {
		msmInstance = MigrationSpanMap{}
		msmInstance.migrationUIDToSpan = make(map[string]opentracing.Span)
	})
	msmInstance.mutex.RLock()
	defer msmInstance.mutex.RUnlock()

	migrationSpan, ok := msmInstance.migrationUIDToSpan[migrationUID]
	if !ok {
		return nil
	}
	return migrationSpan
}

// CloseMigrationSpan closes out the parent Migration Span safely
func CloseMigrationSpan(migrationUID string) {
	migrationSpan := GetSpanForMigrationUID(migrationUID)
	if migrationSpan != nil {
		// Immediately remove span from map so other controllers stop adding child spans
		RemoveSpanForMigrationUID(migrationUID)
		// Wait 5 minutes before terminating span, since other controllers writing to span
		// post-close will result in undefined jaeger behavior (e.g. broken statistics page)
		go func(migrationSpan opentracing.Span) {
			time.Sleep(5 * time.Minute)
			migrationSpan.Finish()
		}(migrationSpan)
	}
}
