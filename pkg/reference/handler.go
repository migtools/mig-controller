package reference

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//
func GetRequests(a client.Object, migrationNamespace string, source interface{}) []reconcile.Request {
	requests := []reconcile.Request{}
	refMap := GetMap()
	refTarget := RefTarget{
		Kind:      ToKind(a),
		Name:      a.GetName(),
		Namespace: a.GetNamespace(),
	}
	owners := refMap.Find(refTarget, RefOwner{Kind: ToKind(source)})
	for i := range owners {
		refOwner := owners[i]
		if refOwner.Namespace == migrationNamespace {
			r := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: refOwner.Namespace,
					Name:      refOwner.Name,
				},
			}
			requests = append(requests, r)
		}
	}
	return requests
}
