package migtoken

import (
	"context"
	"github.com/konveyor/mig-controller/pkg/logging"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/runtime"

	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("token")

var _ reconcile.Reconciler = &ReconcileMigToken{}

// ReconcileMigToken reconciles a MigCluster object
type ReconcileMigToken struct {
	k8sclient.Client
	scheme     *runtime.Scheme
	Controller controller.Controller
}

// Add creates a new MigToken Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	reconciler := &ReconcileMigToken{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
	// Create a new controller
	c, err := controller.New("migtoken-controller", mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Add reference to controller on ReconcileMigCluster object to be used
	// for adding remote watches at a later time
	reconciler.Controller = c

	// Watch for changes to MigToken
	err = c.Watch(
		&source.Kind{Type: &migapi.MigToken{}},
		&handler.EnqueueRequestForObject{},
		&TokenPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to Secrets referenced by MigTokens
	err = c.Watch(
		&source.Kind{Type: &kapi.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigToken{})
				}),
		})
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

func (r *ReconcileMigToken) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var err error
	log.Reset()

	// Fetch the MigCluster
	token := &migapi.MigToken{}
	err = r.Get(context.TODO(), request.NamespacedName, token)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Report reconcile error.
	defer func() {
		if err == nil || errors.IsConflict(err) {
			return
		}
		token.Status.SetReconcileFailed(err)
		err := r.Update(context.TODO(), token)
		if err != nil {
			log.Trace(err)
			return
		}
	}()

	// Begin staging conditions.
	token.Status.BeginStagingConditions()

	err = token.SetTokenStatusFields(r.Client)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Validations.
	err = r.validate(token)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Ready
	token.Status.SetReady(
		!token.Status.HasBlockerCondition(),
		ReadyMessage)

	// End staging conditions.
	token.Status.EndStagingConditions()

	// Apply changes.
	token.MarkReconciled()
	err = r.Update(context.TODO(), token)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{}, nil
}
