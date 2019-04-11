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
	refOwner := RefOwner{
		Kind: ToKind(source),
	}
	requests := []reconcile.Request{}
	sources := refMap.Find(refTarget, refOwner)
	for i := range sources {
		refSource := sources[i]
		r := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: refSource.Namespace,
				Name:      refSource.Name,
			},
		}
		requests = append(requests, r)
	}

	return requests
}
