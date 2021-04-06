package event

import (
	"context"
	"fmt"
	"path"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetAbnormalEventsForResource gets unique events of non-normal type for
// a namespaced resource. Useful for logging the most relevant events
// related to a resource we're waiting on.
// Make sure to pass in the kind exactly as it is capitalized on the event, e.g. "Pod"
func GetAbnormalEventsForResource(client client.Client,
	nsName types.NamespacedName, resourceKind string) ([]corev1.Event, error) {
	uniqueEventMap := make(map[string]corev1.Event)

	eList := corev1.EventList{}
	options := k8sclient.InNamespace(nsName.Namespace)
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s,type!=Normal",
		nsName.Name, resourceKind)
	err := options.SetFieldSelector(fieldSelector)
	if err != nil {
		return nil, fmt.Errorf("field selector construction failed: fieldSelector=[%v]", fieldSelector)
	}
	err = client.List(context.TODO(), options, &eList)
	if err != nil {
		return nil, err
	}
	for _, event := range eList.Items {
		// Check if same event reason has already been seen
		eventFromMap, ok := uniqueEventMap[event.Reason]
		if !ok {
			uniqueEventMap[event.Reason] = event
			continue
		}
		// Found event in map. Overwrite it if this one is newer.
		if eventFromMap.ObjectMeta.CreationTimestamp.Time.
			Before(event.ObjectMeta.CreationTimestamp.Time) {
			uniqueEventMap[event.Reason] = event
		}
	}
	// Turn map into slice of events
	matchingEvents := []corev1.Event{}
	for _, event := range uniqueEventMap {
		matchingEvents = append(matchingEvents, event)
	}

	return matchingEvents, err
}

// LogAbnormalEventsForResource logs unique events of non-normal type for
// a namespaced resource. Useful for logging the most relevant events
// related to a resource we're waiting on.
// The message logged will match what is provided in 'message'
func LogAbnormalEventsForResource(
	client client.Client, log logr.Logger, message string, nsName types.NamespacedName, resourceKind string) {

	relevantEvents, err := GetAbnormalEventsForResource(client,
		types.NamespacedName{Name: nsName.Name, Namespace: nsName.Namespace}, resourceKind)
	if err != nil {
		log.Info("Error getting events",
			"kind", resourceKind,
			"resource", path.Join(nsName.Namespace, nsName.Name),
			"error", err)
		return
	}
	for _, rEvent := range relevantEvents {
		log.Info(message,
			resourceKind, path.Join(nsName.Namespace, nsName.Name),
			"eventType", rEvent.Type,
			"eventReason", rEvent.Reason,
			"eventMessage", rEvent.Message,
			"eventFirstTimestamp", rEvent.FirstTimestamp)
	}

}
