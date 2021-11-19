package migplan

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func MigrationRequests(a client.Object, inNamespace string) []reconcile.Request {
	requests := []reconcile.Request{}
	migration, cast := a.(*migapi.MigMigration)
	if !cast {
		return requests
	}
	ref := migration.Spec.MigPlanRef
	if ref.Namespace != inNamespace {
		return requests
	}
	if migref.RefSet(ref) {
		requests = append(
			requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ref.Namespace,
					Name:      ref.Name,
				},
			})
	}

	return requests
}
