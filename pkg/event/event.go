package event

import (
	"context"
	"path"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetAbnormalEventsForResource gets unique events of non-normal type for
// a namespaced resource. Useful for logging the most relevant events
// related to a resource we're waiting on.
// Make sure to pass in the kind exactly as it is capitalized on the event, e.g. "Pod"
func GetAbnormalEventsForResource(client client.Client,
	resourceNsName types.NamespacedName, resourceUID types.UID) ([]corev1.Event, error) {
	uniqueEventMap := make(map[string]corev1.Event)

	eList := corev1.EventList{}

	options := &k8sclient.ListOptions{
		Namespace: resourceNsName.Namespace,
		FieldSelector: fields.SelectorFromSet(
			fields.Set{
				"involvedObject.uid": string(resourceUID),
			}),
	}
	k8sclient.InNamespace(resourceNsName.Namespace).ApplyToList(options)
	err := client.List(context.TODO(), &eList, options)
	if err != nil {
		return nil, err
	}
	for _, event := range eList.Items {
		// Check if event is a warning
		if event.Type != corev1.EventTypeWarning {
			continue
		}
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
	client client.Client, log logr.Logger, message string,
	resourceNsName types.NamespacedName, resourceUID types.UID, resourceKind string) {

	relevantEvents, err := GetAbnormalEventsForResource(client,
		types.NamespacedName{Name: resourceNsName.Name, Namespace: resourceNsName.Namespace}, resourceUID)
	if err != nil {
		log.Info("Error getting events",
			"kind", resourceKind,
			"resource", path.Join(resourceNsName.Namespace, resourceNsName.Name),
			"error", err)
		return
	}
	for _, rEvent := range relevantEvents {
		log.Info(message,
			resourceKind, path.Join(resourceNsName.Namespace, resourceNsName.Name),
			"eventType", rEvent.Type,
			"eventReason", rEvent.Reason,
			"eventMessage", rEvent.Message,
			"eventFirstTimestamp", rEvent.FirstTimestamp)
	}

}
