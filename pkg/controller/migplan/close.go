/*
Copyright 2019 Red Hat Inc.

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

package migplan

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
)

func (r ReconcileMigPlan) handleClosed(plan *migapi.MigPlan) (bool, error) {
	closed := plan.Spec.Closed
	if !closed || plan.Status.HasCondition(Closed) {
		return closed, nil
	}

	plan.Touch()
	plan.Status.SetReady(false, ReadyMessage)
	err := r.Update(context.TODO(), plan)
	if err != nil {
		return closed, err
	}

	err = r.ensureClosed(plan)
	return closed, err
}

// Ensure that resources managed by the MigPlan have been cleaned up
func (r ReconcileMigPlan) ensureClosed(plan *migapi.MigPlan) error {
	// Migration Registry
	err := r.ensureMigRegistriesDelete(plan)
	if err != nil {
		log.Trace(err)
		return err
	}
	// Storage
	err = r.ensureStorageDeleted(plan)
	if err != nil {
		return err
	}

	plan.Status.DeleteCondition(RegistriesEnsured)
	plan.Status.SetCondition(migapi.Condition{
		Type:     Closed,
		Status:   True,
		Category: Critical,
		Message:  ClosedMessage,
	})
	// Apply changes.
	plan.Touch()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}
