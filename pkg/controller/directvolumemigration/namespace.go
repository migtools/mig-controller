package directvolumemigration

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (t *Task) ensureDestinationNamespaces() error {
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}

	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}

	// Get list namespaces to iterate over
	nsMap := t.getPVCNamespaceMap()
	for ns, _ := range nsMap {
		// Get namespace definition from source cluster
		// This is done to get the needed security context bits

		srcNS := corev1.Namespace{}
		key := types.NamespacedName{Name: ns}
		err = srcClient.Get(context.TODO(), key, &srcNS)
		if err != nil {
			return err
		}

		// Create namespace on destination with same annotations
		destNs := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        ns,
				Annotations: srcNS.Annotations,
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
			return fmt.Errorf("namespace %s is being terminated on destination MigCluster", destNs.Name)
		}
		t.Log.Info("Creating namespace on destination MigCluster",
			"namespace", destNs.Name)
		err = destClient.Create(context.TODO(), &destNs)
		if err != nil {
			if k8serror.IsAlreadyExists(err) {
				t.Log.Info("Namespace already exists on destination MigCluster", "namespace", destNs.Name)
			} else {
				return err
			}
		}
	}
	return nil
}

// Ensure destination namespaces were created
func (t *Task) getDestinationNamespaces() error {
	return nil
}
