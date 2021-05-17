package reference

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func GetRequests(a client.Object, source interface{}) []reconcile.Request {
	refMap := GetMap()
	refTarget := RefTarget{
		Kind:      ToKind(a),
		Name:      a.GetName(),
		Namespace: a.GetNamespace(),
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
