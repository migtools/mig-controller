package miganalytic

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
)

// Types
const (
	InvalidPlanRef = "InvalidPlanRef"
	Postponed      = "Postponed"
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

// Validate the analytic resource.
func (r ReconcileMigAnalytic) validate(analytic *migapi.MigAnalytic) error {
	err := r.validatePlan(analytic)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (r ReconcileMigAnalytic) validatePlan(analytic *migapi.MigAnalytic) error {
	plan := migapi.MigPlan{}
	err := r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      analytic.Spec.MigPlanRef.Name,
			Namespace: analytic.Spec.MigPlanRef.Namespace,
		},
		&plan)

	// NotFound
	if k8serror.IsNotFound(err) {
		analytic.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlanRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  fmt.Sprintf("The referenced plan does not exist."),
		})
		return nil
	} else if err != nil {
		return liberr.Wrap(err)
	}

	//NotReady
	if !plan.Status.IsReady() {
		analytic.Status.SetCondition(migapi.Condition{
			Type:     Postponed,
			Status:   True,
			Reason:   TestFailed,
			Category: Critical,
			Message:  fmt.Sprintf("Waiting %d seconds for referenced MigPlan to become ready.", RequeueInterval),
		})
		return nil
	} else if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}
