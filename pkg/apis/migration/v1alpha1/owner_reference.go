package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func SetOwnerReference(owner metav1.Object, ownerType runtime.Object, dim metav1.Object) {
	trueVar := true
	ownerAPIVersion := ownerType.GetObjectKind().GroupVersionKind().GroupVersion().String()
	ownerKind := ownerType.GetObjectKind().GroupVersionKind().Kind
	ownerUID := owner.GetUID()
	ownerName := owner.GetName()

	for i := range dim.GetOwnerReferences() {
		ref := &dim.GetOwnerReferences()[i]
		if ref.Kind == ownerKind {
			ref.APIVersion = ownerAPIVersion
			ref.Name = ownerName
			ref.UID = ownerUID
			ref.Controller = &trueVar
			return
		}
	}
	dim.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: ownerAPIVersion,
			Kind:       ownerKind,
			Name:       ownerName,
			UID:        ownerUID,
			Controller: &trueVar,
		},
	})
}
