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
	"context"
	"fmt"
	"path"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *Task) getDirectImageMigration() (*migapi.DirectImageMigration, error) {
	dimList := migapi.DirectImageMigrationList{}
	err := t.Client.List(
		context.TODO(),
		k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels()),
		&dimList)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(dimList.Items) > 0 {
		return &dimList.Items[0], nil
	}
	return nil, nil
}

func (t *Task) createDirectImageMigration() error {
	dim, err := t.getDirectImageMigration()
	if err != nil {
		return liberr.Wrap(err)
	}
	if dim != nil {
		return nil
	}
	dim = t.buildDirectImageMigration()
	t.Log.Info("Creating DirectImageMigration resource.",
		"directImageMigration", path.Join(dim.Namespace, dim.Name))
	err = t.Client.Create(context.TODO(), dim)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (t *Task) buildDirectImageMigration() *migapi.DirectImageMigration {
	dim := &migapi.DirectImageMigration{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    t.Owner.Namespace,
		},
		Spec: migapi.DirectImageMigrationSpec{
			SrcMigClusterRef:  t.PlanResources.MigPlan.Spec.SrcMigClusterRef,
			DestMigClusterRef: t.PlanResources.MigPlan.Spec.DestMigClusterRef,
			Namespaces:        t.PlanResources.MigPlan.Spec.Namespaces,
		},
	}
	migapi.SetOwnerReference(t.Owner, t.Owner, dim)
	return dim
}

// Set warning condition on migmigration if DirectImageMigration fails
func (t *Task) setDirectImageMigrationWarning(dim *migapi.DirectImageMigration) {
	if len(dim.Status.Errors) > 0 {
		message := fmt.Sprintf(
			"DirectImageMigration (dim): %s/%s failed. See dim Status.Errors for details.", dim.GetNamespace(), dim.GetName())
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     DirectImageMigrationFailed,
			Status:   True,
			Category: migapi.Warn,
			Message:  message,
			Durable:  true,
		})
	}
}

func (t *Task) deleteDirectImageMigrationResources() error {

	// fetch the DIM
	dim, err := t.getDirectImageMigration()
	if err != nil {
		return liberr.Wrap(err)
	}

	if dim != nil {
		// delete the DIM instance
		t.Log.Info("Deleting DirectImageMigration on host cluster "+
			"due to correlation with MigPlan",
			"directImageMigration", path.Join(dim.Namespace, dim.Name))
		err = t.Client.Delete(context.TODO(), dim)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}
