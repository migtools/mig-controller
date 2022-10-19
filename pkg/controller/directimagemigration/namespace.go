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
	"fmt"

	liberr "github.com/konveyor/controller/pkg/error"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (t *Task) ensureDestinationNamespaces() error {
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	// Get list namespaces to iterate over
	for srcNsName, destNsName := range t.Owner.GetNamespaceMapping() {
		// Get namespace definition from source cluster
		// This is done to get the needed security context bits

		// if namespace exists, don't create it
		foundDestNS := corev1.Namespace{}
		err = destClient.Get(context.TODO(), types.NamespacedName{Name: destNsName}, &foundDestNS)
		if err == nil {
			continue
		}

		srcNS := corev1.Namespace{}
		err = srcClient.Get(context.TODO(), types.NamespacedName{Name: srcNsName}, &srcNS)
		if err != nil {
			return err
		}

		// Create namespace on destination with same annotations
		destNs := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        destNsName,
				Annotations: srcNS.Annotations,
				Labels:      srcNS.Labels,
			},
		}
		existingNamespace := &corev1.Namespace{}
		err = destClient.Get(context.TODO(),
			types.NamespacedName{Name: destNs.Name, Namespace: destNs.Namespace}, existingNamespace)
		if err != nil {
			if !k8serror.IsNotFound(err) {
				return err
			}
		}
		if existingNamespace != nil && existingNamespace.DeletionTimestamp != nil {
			return fmt.Errorf("namespace %s is being terminated on destination MigCluster", destNsName)
		}
		t.Log.Info("Creating namespace on destination MigCluster",
			"namespace", destNs.Name)
		err = destClient.Create(context.TODO(), &destNs)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("Namespace already exists on destination", "name", destNsName)
		} else if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}
