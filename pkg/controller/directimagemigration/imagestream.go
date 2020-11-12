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
		k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels()),
		&dismList)
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
	var isRefList []migapi.ImageStreamListItem
	// Get list namespaces to iterate over
	for srcNsName, destNsName := range t.Owner.GetNamespaceMapping() {
		isList := imagev1.ImageStreamList{}
		err := srcClient.List(
			context.TODO(),
			k8sclient.InNamespace(srcNsName),
			&isList)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, is := range isList.Items {
			isRefList = append(
				isRefList,
				migapi.ImageStreamListItem{
					Name:          is.Name,
					Namespace:     is.Namespace,
					DestNamespace: destNsName,
				},
			)
		}
	}
	t.Owner.Status.ImageStreams = isRefList
	return nil
}
func (t *Task) createDirectImageStreamMigrations() error {
	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	// Get list namespaces to iterate over
	for n, isRef := range t.Owner.Status.ImageStreams {
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
			t.Owner.Status.ImageStreams[n].NotFound = true
		case err != nil:
			return liberr.Wrap(err)
		default:
			t.Owner.Status.ImageStreams[n].NotFound = false
		}
		dismList := migapi.DirectImageStreamMigrationList{}
		err = t.Client.List(
			context.TODO(),
			k8sclient.MatchingLabels(t.Owner.DirectImageStreamMigrationLabels(imageStream)),
			&dismList)
		if err != nil {
			return liberr.Wrap(err)
		}
		if t.Owner.Status.ImageStreams[n].NotFound {
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
	t.setDirectImageStreamOwnerReference(&imageStreamMigration)
	if is.Namespace != destNsName {
		imageStreamMigration.Spec.DestNamespace = destNsName
	}
	return imageStreamMigration
}

func (t *Task) setDirectImageStreamOwnerReference(dism *migapi.DirectImageStreamMigration) {
	trueVar := true
	for i := range dism.OwnerReferences {
		ref := &dism.OwnerReferences[i]
		if ref.Kind == t.Owner.Kind {
			ref.APIVersion = t.Owner.APIVersion
			ref.Name = t.Owner.Name
			ref.UID = t.Owner.UID
			ref.Controller = &trueVar
			return
		}
	}
	dism.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: t.Owner.APIVersion,
			Kind:       t.Owner.Kind,
			Name:       t.Owner.Name,
			UID:        t.Owner.UID,
			Controller: &trueVar,
		},
	}
}
