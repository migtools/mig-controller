/*
Copyright 2020 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package directvolumemigrationprogress

import (
	"context"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/konveyor/mig-controller/pkg/errorutil"
	"github.com/opentracing/opentracing-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("pvmigrationprogress")

var TimeLimit = 10 * time.Minute

const (
	NotFound    = "NotFound"
	NotSet      = "NotSet"
	NotDistinct = "NotDistinct"
	NotReady    = "NotReady"
)

const (
	InvalidClusterRef = "InvalidClusterRef"
	ClusterNotReady   = "ClusterNotReady"
	InvalidPodRef     = "InvalidPodRef"
	InvalidPod        = "InvalidPod"
	PodNotReady       = "PodNotReady"
)

const (
	// CreatingContainer initial container state
	ContainerCreating = "ContainerCreating"
)

// Add creates a new DirectVolumeMigrationProgress Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDirectVolumeMigrationProgress{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("directvolumemigrationprogress-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DirectVolumeMigrationProgress
	err = c.Watch(&source.Kind{Type: &migapi.DirectVolumeMigrationProgress{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by DirectVolumeMigrationProgress - change this for objects you create
	//err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
	//	IsController: true,
	//	OwnerType:    &migrationv1alpha1.DirectVolumeMigrationProgress{},
	//})
	//if err != nil {
	//	return err
	//}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDirectVolumeMigrationProgress{}

// ReconcileDirectVolumeMigrationProgress reconciles a DirectVolumeMigrationProgress object
type ReconcileDirectVolumeMigrationProgress struct {
	client.Client
	scheme *runtime.Scheme
	tracer opentracing.Tracer
}

// Reconcile reads that state of the cluster for a DirectVolumeMigrationProgress object and makes changes based on the state read
// and what is in the DirectVolumeMigrationProgress.Spec

// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directvolumemigrationprogresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directvolumemigrationprogresses/status,verbs=get;update;patch
func (r *ReconcileDirectVolumeMigrationProgress) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var err error
	log.Reset()
	log.SetValues("dvmp", request.Name)
	// Fetch the DirectVolumeMigrationProgress instance
	pvProgress := &migapi.DirectVolumeMigrationProgress{}
	err = r.Get(context.TODO(), request.NamespacedName, pvProgress)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: true}, err
	}

	// Set up jaeger tracing
	reconcileSpan, err := r.initTracer(*pvProgress)
	if reconcileSpan != nil {
		defer reconcileSpan.Finish()
	}

	// Report reconcile error.
	defer func() {
		if err == nil || errors.IsConflict(errorutil.Unwrap(err)) {
			return
		}
		pvProgress.Status.SetReconcileFailed(err)
		err := r.Update(context.TODO(), pvProgress)
		if err != nil {
			log.Trace(err)
			return
		}
	}()

	// Begin staging conditions.
	pvProgress.Status.BeginStagingConditions()

	err = r.reportContainerStatus(pvProgress, "rsync-client")
	if err != nil {
		return reconcile.Result{Requeue: true}, liberr.Wrap(err)
	}

	pvProgress.Status.SetReady(!pvProgress.Status.HasCriticalCondition(), "The progress is available")
	if pvProgress.Status.HasCriticalCondition() {
		pvProgress.Status.PodPhase = ""
	}

	pvProgress.Status.EndStagingConditions()

	pvProgress.MarkReconciled()
	err = r.Update(context.TODO(), pvProgress)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// we will requeue this every 5 seconds
	return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, nil
}

func (r *ReconcileDirectVolumeMigrationProgress) reportContainerStatus(pvProgress *migapi.DirectVolumeMigrationProgress, containerName string) error {
	podRef := pvProgress.Spec.PodRef
	ref := pvProgress.Spec.ClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     InvalidClusterRef,
			Status:   migapi.True,
			Reason:   NotSet,
			Category: migapi.Critical,
			Message:  "The spec.clusterRef must reference name and namespace of a valid `MigCluster",
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// NotFound
	if cluster == nil {
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     InvalidClusterRef,
			Status:   migapi.True,
			Reason:   NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The spec.clusterRef must reference a valid `MigCluster` %s",
				path.Join(ref.Namespace, ref.Name)),
		})
		return nil
	}

	// Not ready
	if !cluster.Status.IsReady() {
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     ClusterNotReady,
			Status:   migapi.True,
			Reason:   NotReady,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The `MigCluster` spec.ClusterRef %s is not ready",
				path.Join(ref.Namespace, ref.Name)),
		})
	}

	pod, err := r.Pod(cluster, podRef)
	switch {
	case errors.IsNotFound(err):
		// handle not found and return
		pvProgress.Status.SetCondition(migapi.Condition{
			Type:     InvalidPod,
			Status:   migapi.True,
			Reason:   NotFound,
			Category: migapi.Critical,
			Message: fmt.Sprintf("The spec.podRef %s must reference a valid `Pod` ",
				path.Join(podRef.Namespace, podRef.Name)),
		})
		return nil
	case err != nil:
		return liberr.Wrap(err)
	}

	var containerStatus *kapi.ContainerStatus
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == containerName {
			containerStatus = &c
		}
	}

	if containerStatus == nil {
		if pod.Status.Phase == kapi.PodFailed {
			pvProgress.Status.PodPhase = kapi.PodFailed
			pvProgress.Status.LogMessage = pod.Status.Message
			pvProgress.Status.ContainerElapsedTime = nil
		} else {
			pvProgress.Status.SetCondition(migapi.Condition{
				Type:     InvalidPod,
				Status:   migapi.True,
				Reason:   NotFound,
				Category: migapi.Critical,
				Message: fmt.Sprintf("The spec.podRef %s must reference a `Pod` with container name %s",
					path.Join(podRef.Namespace, podRef.Name), containerName),
			})
		}
		return nil
	}

	switch {
	case containerStatus.Ready:
		// report pod running and return
		pvProgress.Status.PodPhase = kapi.PodRunning
		numberOfLogLines := int64(5)
		logMessage, err := r.GetPodLogs(cluster, podRef, &numberOfLogLines, false)
		if err != nil {
			return err
		}
		pvProgress.Status.LogMessage = logMessage
		percentProgress := GetProgressPercent(logMessage)
		if percentProgress != "" {
			pvProgress.Status.LastObservedProgressPercent = percentProgress
		}
		transferRate := GetTransferRate(logMessage)
		if transferRate != "" {
			pvProgress.Status.LastObservedTransferRate = transferRate
		}
		pvProgress.Status.ContainerElapsedTime = nil
	case !containerStatus.Ready && containerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated.ExitCode != 0:
		// pod has a failure, report last failure reason
		pvProgress.Status.PodPhase = kapi.PodFailed
		pvProgress.Status.LogMessage = containerStatus.LastTerminationState.Terminated.Message
		exitCode := containerStatus.LastTerminationState.Terminated.ExitCode
		pvProgress.Status.ExitCode = &exitCode
		pvProgress.Status.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.LastTerminationState.Terminated.FinishedAt.Sub(containerStatus.LastTerminationState.Terminated.StartedAt.Time).Round(time.Second)}
	case !containerStatus.Ready && pod.Status.Phase == kapi.PodPending && containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == ContainerCreating:
		// if pod is not in running state after 10 mins of its creation, raise a
		if time.Now().UTC().Sub(pod.CreationTimestamp.Time.UTC()) > TimeLimit {
			pvProgress.Status.PodPhase = kapi.PodPending
			pvProgress.Status.LogMessage = fmt.Sprintf("Pod %s/%s is stuck in Pending state for more than 10 mins", pod.Namespace, pod.Name)
			pvProgress.Status.ContainerElapsedTime = &metav1.Duration{Duration: time.Now().Sub(pod.CreationTimestamp.Time).Round(time.Second)}
		}
	case pod.Status.Phase == kapi.PodFailed:
		// Its possible for the succeeded pod to not have containerStatuses at all
		pvProgress.Status.PodPhase = kapi.PodFailed
		pvProgress.Status.LogMessage = containerStatus.State.Terminated.Message
		exitCode := containerStatus.State.Terminated.ExitCode
		pvProgress.Status.ExitCode = &exitCode
		pvProgress.Status.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.State.Terminated.FinishedAt.Sub(containerStatus.State.Terminated.StartedAt.Time).Round(time.Second)}
	case !containerStatus.Ready && containerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated.ExitCode == 0:
		// succeeded dont ever requeue
		pvProgress.Status.PodPhase = kapi.PodSucceeded
		pvProgress.Status.LastObservedProgressPercent = "100%"
		exitCode := containerStatus.LastTerminationState.Terminated.ExitCode
		pvProgress.Status.ExitCode = &exitCode
		pvProgress.Status.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.LastTerminationState.Terminated.FinishedAt.Sub(containerStatus.LastTerminationState.Terminated.StartedAt.Time).Round(time.Second)}
	case pod.Status.Phase == kapi.PodSucceeded:
		// Its possible for the succeeded pod to not have containerStatuses at all
		pvProgress.Status.PodPhase = kapi.PodSucceeded
		pvProgress.Status.LastObservedProgressPercent = "100%"
		exitCode := containerStatus.State.Terminated.ExitCode
		pvProgress.Status.ExitCode = &exitCode
		pvProgress.Status.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.State.Terminated.FinishedAt.Sub(containerStatus.State.Terminated.StartedAt.Time).Round(time.Second)}
	}

	return nil
}

func (r *ReconcileDirectVolumeMigrationProgress) Pod(cluster *migapi.MigCluster, podReference *kapi.ObjectReference) (*kapi.Pod, error) {
	cli, err := cluster.GetClient(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	pod := &kapi.Pod{}
	err = cli.Get(context.TODO(), types.NamespacedName{
		Namespace: podReference.Namespace,
		Name:      podReference.Name,
	}, pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func (r *ReconcileDirectVolumeMigrationProgress) GetPodLogs(cluster *migapi.MigCluster, podReference *kapi.ObjectReference, tailLines *int64, previous bool) (string, error) {

	config, err := cluster.BuildRestConfig(r.Client)
	if err != nil {
		return "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	req := clientset.CoreV1().Pods(podReference.Namespace).GetLogs(podReference.Name, &kapi.PodLogOptions{
		TailLines: tailLines,
		Previous:  previous,
		Container: "rsync-client",
	})
	readCloser, err := req.Stream()
	if err != nil {
		return "", err
	}

	defer readCloser.Close()

	return parseLogs(readCloser)
}

// GetProgressPercent given logs from Rsync Pod, returns logged progress percentage
func GetProgressPercent(message string) string {
	return getLastMatch(`\d+\%`, message)
}

// GetTransferRate given logs from Rsync Pod, returns logged transfer rate
func GetTransferRate(message string) string {
	return getLastMatch(`\d+\.\w*\/s`, message)
}

// ProgressPercentageToQuantity parses string and returns percentage as a value
func ProgressPercentageToQuantity(progressPercentage string) *int64 {
	var value int64
	r := regexp.MustCompile(`(\d+)\%`)
	matched := r.FindStringSubmatch(progressPercentage)
	if len(matched) == 2 {
		v, err := strconv.ParseInt(matched[1], 10, 64)
		if err == nil {
			value = v
		}
	}
	return &value
}

// ProgressPercentageToString parses string and returns percentage as a value
func ProgressPercentageToString(progressPercentage *int64) string {
	if progressPercentage == nil {
		return ""
	}
	return fmt.Sprintf("%d%%", progressPercentage)
}

func getLastMatch(regex string, message string) string {
	r := regexp.MustCompile(regex)
	matches := r.FindAllString(message, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1]
	}
	return ""
}

func parseLogs(reader io.Reader) (string, error) {
	buf := new(strings.Builder)
	_, err := io.Copy(buf, reader)
	if err != nil {
		return "", err
	}
	data := strings.Split(buf.String(), "\n")
	logLines := []string{}
	for _, line := range data {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if len(l) > 60 {
			l = l[:60]
		}
		logLines = append(logLines, l)
	}
	return strings.Join(logLines, "\n"), nil
}
