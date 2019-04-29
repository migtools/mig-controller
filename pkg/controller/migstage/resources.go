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

package migstage

import (
	"fmt"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/fusor/mig-controller/pkg/migshared"
)

// getReconcileResources puts together a struct with all resources needed to perform a MigStage reconcile.
func (r *ReconcileMigStage) getReconcileResources(migStage *migapi.MigStage) (*migshared.ReconcileResources, error) {
	migPlan, err := migStage.GetPlan(r.Client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET MigPlan referenced by MigStage [%s/%s]",
			logPrefix, migStage.Namespace, migStage.Name))
		return nil, err
	}

	resources, err := migshared.GetReconcileResources(r.Client, migPlan, logPrefix)
	return resources, err
}
