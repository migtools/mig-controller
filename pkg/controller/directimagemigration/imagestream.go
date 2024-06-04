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
	"context"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	imagev1 "github.com/openshift/api/image/v1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *Task) getDirectImageStreamMigrations() ([]migapi.DirectImageStreamMigration, error) {
	dismList := migapi.DirectImageStreamMigrationList{}
	err := t.Client.List(
		context.TODO(),
		&dismList,
		k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels()))
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return dismList.Items, nil
}

func (t *Task) listImageStreams() error {
	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	var isRefList []*migapi.ImageStreamListItem
	// Get list namespaces to iterate over
	for srcNsName, destNsName := range t.Owner.GetNamespaceMapping() {
		labels, _ := metav1.LabelSelectorAsSelector(t.Owner.Spec.LabelSelector)
		isList := imagev1.ImageStreamList{}
		err := srcClient.List(
			context.TODO(),
			&isList,
			k8sclient.InNamespace(srcNsName),
			k8sclient.MatchingLabelsSelector{Selector: labels},
		)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, is := range isList.Items {
			objRef := &kapi.ObjectReference{
				Namespace: is.Namespace,
				Name:      is.Name,
			}
			isRefList = append(
				isRefList,
				&migapi.ImageStreamListItem{
					ObjectReference: objRef,
					DestNamespace:   destNsName,
				},
			)
		}
	}
	t.Owner.Status.NewISs = isRefList
	return nil
}
func (t *Task) createDirectImageStreamMigrations() error {
	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	// Get list namespaces to iterate over
	for n, isRef := range t.Owner.Status.NewISs {
		if isRef.DirectMigration != nil {
			continue
		}
		imageStream := imagev1.ImageStream{}
		err := srcClient.Get(
			context.TODO(),
			types.NamespacedName{
				Namespace: isRef.Namespace,
				Name:      isRef.Name,
			},
			&imageStream)
		switch {
		case errors.IsNotFound(err):
			t.Owner.Status.NewISs[n].NotFound = true
		case err != nil:
			return liberr.Wrap(err)
		default:
			t.Owner.Status.NewISs[n].NotFound = false
		}
		dismList := migapi.DirectImageStreamMigrationList{}
		err = t.Client.List(
			context.TODO(),
			&dismList,
			k8sclient.MatchingLabels(t.Owner.DirectImageStreamMigrationLabels(imageStream)))
		if err != nil {
			return liberr.Wrap(err)
		}
		if t.Owner.Status.NewISs[n].NotFound {
			for _, dism := range dismList.Items {
				// Delete
				err := t.Client.Delete(context.TODO(), &dism)
				if err != nil && !errors.IsNotFound(err) {
					return liberr.Wrap(err)
				}
			}
			continue
		}
		if len(dismList.Items) > 0 {
			continue
		}
		imageStreamMigration := t.buildDirectImageStreamMigration(imageStream, isRef.DestNamespace)
		err = t.Client.Create(context.TODO(), &imageStreamMigration)
		if err != nil {
			return liberr.Wrap(err)
		}
		objRef := &kapi.ObjectReference{
			Namespace: imageStreamMigration.Namespace,
			Name:      imageStreamMigration.Name,
		}
		t.Owner.Status.NewISs[n].DirectMigration = objRef

	}
	return nil
}
func (t *Task) buildDirectImageStreamMigration(is imagev1.ImageStream, destNsName string) migapi.DirectImageStreamMigration {
	labels := t.Owner.DirectImageStreamMigrationLabels(is)
	imageStreamMigration := migapi.DirectImageStreamMigration{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       labels,
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    t.Owner.Namespace,
		},
		Spec: migapi.DirectImageStreamMigrationSpec{
			SrcMigClusterRef:  t.Owner.Spec.SrcMigClusterRef,
			DestMigClusterRef: t.Owner.Spec.DestMigClusterRef,
			ImageStreamRef: &kapi.ObjectReference{
				Name:      is.Name,
				Namespace: is.Namespace,
			},
		},
	}
	migapi.SetOwnerReference(t.Owner, t.Owner, &imageStreamMigration)
	if is.Namespace != destNsName {
		imageStreamMigration.Spec.DestNamespace = destNsName
	}
	return imageStreamMigration
}

func (t *Task) checkDISMCompletion() (bool, []string) {
	newISs := []*migapi.ImageStreamListItem{}
	for _, item := range t.Owner.Status.NewISs {
		if item.NotFound {
			t.Owner.Status.DeletedISs = append(t.Owner.Status.DeletedISs, item)
			continue
		}
		dism := migapi.DirectImageStreamMigration{}
		err := t.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Namespace: item.DirectMigration.Namespace,
				Name:      item.DirectMigration.Name,
			},
			&dism)
		// If retrieving the dism failed, consider that the associated ImageStream failed migration
		if err != nil {
			item.Errors = append(item.Errors, err.Error())
			t.Owner.Status.FailedISs = append(t.Owner.Status.FailedISs, item)
			continue
		}
		dismCompleted, dismErrors := dism.HasCompleted()
		switch {
		case dismCompleted && len(dismErrors) == 0:
			t.Owner.Status.SuccessfulISs = append(t.Owner.Status.SuccessfulISs, item)
		case dismCompleted:
			item.Errors = append(item.Errors, dismErrors...)
			t.Owner.Status.FailedISs = append(t.Owner.Status.FailedISs, item)
		default:
			newISs = append(newISs, item)
		}
	}
	t.Owner.Status.NewISs = newISs

	completed := len(t.Owner.Status.NewISs) == 0
	reasons := []string{}
	if completed {
		for _, item := range t.Owner.Status.FailedISs {
			reasons = append(reasons, item.Errors...)
		}
	}
	return completed, reasons
}
