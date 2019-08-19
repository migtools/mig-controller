package event

import (
	"context"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)



func Post(c client.Client, newEvent *v1.Event) error {
	now := metav1.NewTime(time.Now())
	event := &v1.Event{}
	err := c.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: newEvent.Namespace,
			Name: newEvent.Name,
		},
		event)
	if err == nil {
		event.Count++
		event.LastTimestamp = now
		event.Reason = newEvent.Reason
		event.Message = newEvent.Message
		return c.Update(context.TODO(), event)
	}
	if errors.IsNotFound(err) {
		event = newEvent
		event.Count = 1
		event.FirstTimestamp = now
		event.LastTimestamp = now
		return c.Create(context.TODO(), event)
	}

	return err
}
