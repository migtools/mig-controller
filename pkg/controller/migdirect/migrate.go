package migdirect

import (
	"fmt"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *ReconcileMigDirect) migrate(direct *migapi.MigDirect) (time.Duration, error) {
	// Started
	if direct.Status.StartTimestamp == nil {
		direct.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	}

	// Run
	task := Task{
		Log:    log,
		Client: r,
		Owner:  direct,
		Phase:  direct.Status.Phase,
	}
	err := task.Run()
	if err != nil {
		if errors.IsConflict(err) {
			return FastReQ, nil
		}
		log.Trace(err)
		task.fail(MigrationFailed, []string{err.Error()})
		return task.Requeue, nil
	}

	// Result
	direct.Status.Phase = task.Phase
	direct.Status.Itinerary = task.Itinerary.Name

	// Completed
	if task.Phase == Completed {
		direct.Status.DeleteCondition(Running)
		failed := task.Owner.Status.FindCondition(Failed)
		if failed == nil {
			direct.Status.SetCondition(migapi.Condition{
				Type:     Succeeded,
				Status:   True,
				Reason:   task.Phase,
				Category: Advisory,
				Message:  SucceededMessage,
				Durable:  true,
			})
		}
		return NoReQ, nil
	}

	// Running
	step, n, total := task.Itinerary.progressReport(task.Phase)
	message := fmt.Sprintf(RunningMessage, n, total)
	direct.Status.SetCondition(migapi.Condition{
		Type:     Running,
		Status:   True,
		Reason:   step,
		Category: Advisory,
		Message:  message,
	})

	return task.Requeue, nil
}
