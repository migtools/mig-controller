package reference

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func GetRequests(a handler.MapObject, source interface{}) []reconcile.Request {
	refMap := GetMap()
	refTarget := RefTarget{
		Kind:      ToKind(a.Object),
		Name:      a.Meta.GetName(),
		Namespace: a.Meta.GetNamespace(),
	}
	requests := []reconcile.Request{}
	owners := refMap.Find(refTarget, RefOwner{Kind: ToKind(source)})
	for i := range owners {
		refOwner := owners[i]
		r := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: refOwner.Namespace,
				Name:      refOwner.Name,
			},
		}
		requests = append(requests, r)
	}

	return requests
}
