package remotewatcher

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	kapi "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type SecretPredicate struct {
	predicate.Funcs
}

// Watched resource has been updated.
func (r SecretPredicate) Update(e event.UpdateEvent) bool {
	secret, cast := e.ObjectOld.(*kapi.Secret)
	if cast {
		return hasCorrelationLabel(secret)
	}
	return false
}

// Watched resource has been deleted.
func (r SecretPredicate) Delete(e event.DeleteEvent) bool {
	secret, cast := e.Object.(*kapi.Secret)
	if cast {
		return hasCorrelationLabel(secret)
	}
	return false
}

// The watched object has a correlation label.
func hasCorrelationLabel(secret *kapi.Secret) bool {
	for label := range secret.Labels {
		_, found := migapi.KnownLabels[label]
		if found {
			return true
		}
	}
	return false
}
