package pods

import (
	"context"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Find all velero pods for the specified client.
func FindVeleroPods(client k8sclient.Client) ([]corev1.Pod, error) {
	var podList []corev1.Pod
	list := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(
		map[string]string{
			"component": "velero",
		})
	fieldSelector := fields.SelectorFromSet(
		map[string]string{
			"status.phase": "Running",
		})
	err := client.List(
		context.TODO(),
		list,
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		})
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, pod := range list.Items {
		if pod.DeletionTimestamp == nil {
			podList = append(podList, pod)
		}
	}
	return podList, nil
}
