package directvolumemigration

import (
	"context"
	corev1 "k8s.io/api/core/v1"
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
		// Remove openshift node-selector label
		newAnnotations := srcNS.Annotations
		delete(newAnnotations, "openshift.io/node-selector")

		// Create namespace on destination with same annotations
		destNs := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        ns,
				Annotations: newAnnotations,
			},
		}
		err = destClient.Create(context.TODO(), &destNs)
		if err != nil {
			return err
		}
	}
	return nil
}

// Ensure destination namespaces were created
func (t *Task) getDestinationNamespaces() error {
	return nil
}
