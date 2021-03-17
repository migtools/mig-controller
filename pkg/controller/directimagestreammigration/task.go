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

package directimagestreammigration

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Requeue
var FastReQ = time.Duration(time.Millisecond * 100)
var PollReQ = time.Duration(time.Second * 3)
var NoReQ = time.Duration(0)

// Phases
const (
	Created            = ""
	Started            = "Started"
	Prepare            = "Prepare"
	MigrateImageStream = "MigrateImageStream"
	Completed          = "Completed"
	MigrationFailed    = "MigrationFailed"
)

// Step
type Step struct {
	// A phase name.
	phase string
	// Step included when ALL flags evaluate true.
	all uint8
	// Step included when ANY flag evaluates true.
	any uint8
}

type Itinerary struct {
	Name  string
	Steps []Step
}

// Get a progress report.
// Returns: phase, n, total.
func (r Itinerary) progressReport(phase string) (string, int, int) {
	n := 0
	total := len(r.Steps)
	for i, step := range r.Steps {
		if step.phase == phase {
			n = i + 1
			break
		}
	}

	return phase, n, total
}

var ImageItinerary = Itinerary{
	Name: "PVC",
	Steps: []Step{
		{phase: Created},
		{phase: Started},
		{phase: Prepare},
		{phase: MigrateImageStream},
		{phase: Completed},
	},
}

var FailedItinerary = Itinerary{
	Name: "Failed",
	Steps: []Step{
		{phase: MigrationFailed},
		{phase: Completed},
	},
}

// A task that provides the complete migration workflow.
// Log - A controller's logger.
// Client - A controller's (local) client.
// Owner - A DirectImageMigration resource.
// Phase - The task phase.
// Requeue - The requeueAfter duration. 0 indicates no requeue.
// Itinerary - The phase itinerary.
// Errors - Migration errors.
// Failed - Task phase has failed.
type Task struct {
	Log       logr.Logger
	Client    k8sclient.Client
	Owner     *migapi.DirectImageStreamMigration
	Phase     string
	Requeue   time.Duration
	Itinerary Itinerary
	Errors    []string
}

func (t *Task) init() error {
	t.Requeue = FastReQ
	if t.failed() {
		t.Itinerary = FailedItinerary
	} else {
		t.Itinerary = ImageItinerary
	}
	return nil
}

func (t *Task) Run() error {
	t.Log = t.Log.WithValues("Phase", t.Phase)
	// Init
	err := t.init()
	if err != nil {
		return err
	}

	// Log '[RUN] (Step 12/37) <Extended Phase Description>'
	t.logRunHeader()

	// Run the current phase.
	switch t.Phase {
	case Created, Started:
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case Prepare:
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case MigrateImageStream:
		// Migrate internal images in the imagestream
		err := t.migrateInternalImages()
		if err != nil {
			t.fail(MigrationFailed, []string{err.Error()})
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case Completed:
	default:
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	}

	if t.Phase == Completed {
		t.Requeue = NoReQ
		t.Log.Info("[COMPLETED]")
	}

	return nil
}

// Advance the task to the next phase.
func (t *Task) next() error {
	// Write time taken to complete phase
	t.Owner.Status.Conditions.StageCondition(migapi.Running)
	cond := t.Owner.Status.FindCondition(migapi.Running)
	if cond != nil {
		now := time.Now().UTC()
		elapsed := now.Sub(cond.LastTransitionTime.Time.UTC())
		t.Log.Info("[NEXT]", "Elapsed", elapsed)
	}
	current := -1
	for i, step := range t.Itinerary.Steps {
		if step.phase != t.Phase {
			continue
		}
		current = i
		break
	}
	if current == -1 {
		t.Phase = Completed
		return nil
	}
	for n := current + 1; n < len(t.Itinerary.Steps); n++ {
		next := t.Itinerary.Steps[n]
		t.Phase = next.phase
		return nil
	}
	t.Phase = Completed
	return nil
}

// Phase fail.
func (t *Task) fail(nextPhase string, reasons []string) {
	t.addErrors(reasons)
	t.Owner.AddErrors(t.Errors)
	t.Owner.Status.SetCondition(migapi.Condition{
		Type:     migapi.Failed,
		Status:   migapi.True,
		Reason:   t.Phase,
		Category: migapi.Advisory,
		Message:  "The migration has failed.  See: Errors.",
		Durable:  true,
	})
	t.Phase = nextPhase
}

// Add errors.
func (t *Task) addErrors(errors []string) {
	for _, error := range errors {
		t.Errors = append(t.Errors, error)
	}
}

// Get whether the migration has failed
func (t *Task) failed() bool {
	return t.Owner.HasErrors() || t.Owner.Status.HasCondition(migapi.Failed)
}

// Get client for source cluster
func (t *Task) getSourceClient() (compat.Client, error) {
	cluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return nil, err
	}
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Get client for destination cluster
func (t *Task) getDestinationClient() (compat.Client, error) {
	cluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return nil, err
	}
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Get the extended phase description for a phase.
func (t *Task) getPhaseDescription(phaseName string) string {
	// Log the extended description of current phase
	if phaseDescription, found := PhaseDescriptions[t.Phase]; found {
		return phaseDescription
	}
	t.Log.Info("Missing phase description for phase: " + phaseName)
	// If no description available, just return phase name.
	return phaseName
}

// Log the "[RUN] (Step 12/37) <Phase description>" phase kickoff string
// This is meant to cut down on log noise when two controllers
// are waiting on the same thing.
func (t *Task) logRunHeader() {
	_, n, total := t.Itinerary.progressReport(t.Phase)
	t.Log.Info(fmt.Sprintf("[RUN] (Step %v/%v) %v", n, total, t.getPhaseDescription(t.Phase)))
}
