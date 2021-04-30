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
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/mig-controller/pkg/errorutil"
	"github.com/opentracing/opentracing-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
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

const (
	// CreatingContainer initial container state
	ContainerCreating = "ContainerCreating"
)

const (
	DefaultReconcileConcurrency = 5
	RsyncContainerName          = "rsync-client"
)

type GetPodLogger interface {
	getPodLogs(pod *kapi.Pod, containerName string, tailLines *int64, previous bool) (string, error)
}

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
	c, err := controller.New("directvolumemigrationprogress-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: DefaultReconcileConcurrency})
	if err != nil {
		return err
	}

	// Watch for changes to DirectVolumeMigrationProgress
	err = c.Watch(&source.Kind{Type: &migapi.DirectVolumeMigrationProgress{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

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

	// Set MigMigration name key on logger
	migration, err := pvProgress.GetMigrationforDVMP(r)
	if migration != nil {
		log.SetValues("migMigration", migration.Name)
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

	// Validate
	cluster, srcClient, err := r.validate(pvProgress)
	if err != nil {
		log.V(4).Info("Validation failed, requeueing", "error", err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Analyze pod(s)
	if !pvProgress.Status.HasBlockerCondition() {
		task := &RsyncPodProgressTask{
			Cluster:   cluster,
			Client:    r.Client,
			SrcClient: srcClient,
			Owner:     pvProgress,
		}
		err = task.Run()
		if err != nil {
			return reconcile.Result{Requeue: true}, liberr.Wrap(err)
		}
	}

	// set ready
	pvProgress.Status.SetReady(!pvProgress.Status.HasBlockerCondition(), "The progress is ready")

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

type RsyncPodProgressTask struct {
	Cluster   *migapi.MigCluster
	Client    client.Client
	SrcClient compat.Client
	Owner     *migapi.DirectVolumeMigrationProgress
}

func (r *RsyncPodProgressTask) Run() error {
	pvProgress, podRef, podSelector, podNamespace := r.Owner, r.Owner.Spec.PodRef, r.Owner.Spec.PodSelector, r.Owner.Spec.PodNamespace
	if podRef != nil && podSelector == nil {
		pod, err := getPod(r.SrcClient, podRef)
		if err != nil {
			return liberr.Wrap(err)
		}
		rsyncPodStatus := r.getRsyncClientContainerStatus(pod, r)

		if rsyncPodStatus != nil {
			pvProgress.Status.RsyncPodStatus = *rsyncPodStatus
		}
	} else if podSelector != nil && podNamespace != "" {
		podList, err := r.getAllMatchingRsyncPods()
		if err != nil {
			return liberr.Wrap(err)
		}
		var mostRecentPodStatus *migapi.RsyncPodStatus
		for i := range podList.Items {
			pod := &podList.Items[i]
			// if already part of the history, skip
			// we only add dead pods in the history
			if pvProgress.Status.RsyncPodExistsInHistory(pod.Name) {
				continue
			}
			rsyncPodStatus := r.getRsyncClientContainerStatus(pod,r)
			if rsyncPodStatus != nil {
				// dead pods go in history
				if IsPodTerminal(rsyncPodStatus.PodPhase) {
					err := r.addDVMPDoneLabel(pod)
					if err != nil {
						continue
					}
					// merge progress stats from previous run
					MergeProgressStats(rsyncPodStatus, &pvProgress.Status.RsyncPodStatus)
					pvProgress.Status.RsyncPodStatuses = append(pvProgress.Status.RsyncPodStatuses, *rsyncPodStatus)
				}
				// update mostRecentPodStatus if current pod is more recent
				if mostRecentPodStatus == nil ||
					rsyncPodStatus.CreationTimestamp != nil &&
						mostRecentPodStatus.CreationTimestamp != nil &&
						!mostRecentPodStatus.CreationTimestamp.After(rsyncPodStatus.CreationTimestamp.Time) {
					mostRecentPodStatus = rsyncPodStatus
				}
			}
		}
		// update the top level status field to match the newly found latest pod status
		if mostRecentPodStatus != nil {
			pvProgress.Status.PodName = mostRecentPodStatus.PodName
			pvProgress.Status.PodPhase = mostRecentPodStatus.PodPhase
			pvProgress.Status.LogMessage = mostRecentPodStatus.LogMessage
			pvProgress.Status.ContainerElapsedTime = mostRecentPodStatus.ContainerElapsedTime
			pvProgress.CreationTimestamp = *mostRecentPodStatus.CreationTimestamp
			MergeProgressStats(&pvProgress.Status.RsyncPodStatus, mostRecentPodStatus)
		}
		// update total progress percentage
		r.updateCumulativeProgressPercentage()
		// update total elapsed time
		r.updateCumulativeElapsedTime()
	}
	return nil
}

func (r *RsyncPodProgressTask) addDVMPDoneLabel(pod *kapi.Pod) error {
	podRef := &kapi.Pod{}
	err := r.SrcClient.Get(context.TODO(),
		types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, podRef)
	if err != nil {
		log.Info("Failed to find Pod before adding DVMP label",
			"pod", path.Join(pod.Namespace, pod.Name))
		return err
	}
	// for terminal pods, indicate that we are done collecting information, can safely garbage collect
	podRef.Labels[migapi.DVMPDoneLabelKey] = migapi.True
	err = r.SrcClient.Update(context.TODO(), podRef)
	if err != nil {
		log.Info("Failed to add DVMP label on the Pod",
			"pod", path.Join(pod.Namespace, pod.Name))
		return err
	}
	return nil
}

// MergeProgressStats merges progress stats of p2 into p1 : p1 <- p2, only if p1 & p2 are for same pods
func MergeProgressStats(p1, p2 *migapi.RsyncPodStatus) {
	if p1 == nil || p2 == nil || p1.PodName != p2.PodName {
		return
	}
	getNonEmpty := func(s1, s2 string) string {
		if s2 == "" {
			return s1
		} else {
			return s2
		}
	}
	getNonNil := func(i1, i2 *int32) *int32 {
		if i1 == nil {
			return i2
		} else {
			return i1
		}
	}
	p1.ExitCode = getNonNil(p1.ExitCode, p2.ExitCode)
	p1.LastObservedProgressPercent = MaxProgressString(p1.LastObservedProgressPercent, p2.LastObservedProgressPercent)
	p1.LastObservedTransferRate = getNonEmpty(p1.LastObservedTransferRate, p2.LastObservedTransferRate)
}

func IsPodTerminal(phase kapi.PodPhase) bool {
	if phase == kapi.PodFailed ||
		phase == kapi.PodSucceeded {
		return true
	}
	return false
}

// updateCumulativeProgressPercentage computes overall progress percentage of Rsync
func (r *RsyncPodProgressTask) updateCumulativeProgressPercentage() {
	totalProgress := int64(0)
	for _, podHistory := range r.Owner.Status.RsyncPodStatuses {
		if podHistory.PodName != r.Owner.Status.PodName {
			totalProgress += ProgressStringToValue(podHistory.LastObservedProgressPercent)
		}
	}
	totalProgress += ProgressStringToValue(r.Owner.Status.LastObservedProgressPercent)
	totalProgress = int64(math.Min(float64(totalProgress), float64(100)))
	r.Owner.Status.TotalProgressPercentage = ProgressValueToString(totalProgress)
}

// updateCumulativeElapsedTime computes overall elapsed time of Rsync
func (r *RsyncPodProgressTask) updateCumulativeElapsedTime() {
	currentTime, newTime := metav1.Now(), metav1.Now()
	totalElapsedDuration := metav1.Duration{}
	for _, podHistory := range r.Owner.Status.RsyncPodStatuses {
		if podHistory.PodName != r.Owner.Status.PodName && podHistory.ContainerElapsedTime != nil {
			newTime.Time = newTime.Add(podHistory.ContainerElapsedTime.Duration)
		}
	}
	if r.Owner.Status.ContainerElapsedTime != nil {
		newTime.Time = newTime.Add(r.Owner.Status.ContainerElapsedTime.Duration)
	}
	totalElapsedDuration.Duration = newTime.Sub(currentTime.Time).Round(time.Second)
	r.Owner.Status.RsyncElapsedTime = &totalElapsedDuration
}

// getRsyncClientContainerStatus returns observed status of Rsync container in the given pod
// podLogGetterFunction is a function capable of retrieving logs from a given pod, injected to make testing easier
func (r *RsyncPodProgressTask) getRsyncClientContainerStatus(podRef *kapi.Pod, p GetPodLogger) *migapi.RsyncPodStatus {
	rsyncPodStatus := migapi.RsyncPodStatus{
		PodName:           podRef.Name,
		CreationTimestamp: &podRef.CreationTimestamp,
		PodPhase:          podRef.Status.Phase,
		LogMessage:        podRef.Status.Message,
	}
	var containerStatus *kapi.ContainerStatus
	for i := range podRef.Status.ContainerStatuses {
		c := podRef.Status.ContainerStatuses[i]
		if c.Name == RsyncContainerName {
			containerStatus = &c
		}
	}
	if containerStatus == nil {
		log.Info("Failed to find container in Rsync Pod on source cluster",
			"container", RsyncContainerName, "pod", path.Join(podRef.Namespace, podRef.Name))
		return &rsyncPodStatus
	}
	switch {
	case containerStatus.Ready:
		rsyncPodStatus.PodPhase = kapi.PodRunning
		numberOfLogLines := int64(5)
		logMessage, err := p.getPodLogs(podRef, RsyncContainerName, &numberOfLogLines, false)
		if err != nil {
			log.Info("Failed to get logs from Rsync Pod on source cluster",
				"pod", path.Join(podRef.Namespace, podRef.Name))
			return &rsyncPodStatus
		}
		rsyncPodStatus.LogMessage = logMessage
		percentProgress := GetProgressPercent(logMessage)
		if percentProgress != "" {
			rsyncPodStatus.LastObservedProgressPercent = percentProgress
		}
		transferRate := GetTransferRate(logMessage)
		if transferRate != "" {
			rsyncPodStatus.LastObservedTransferRate = transferRate
		}
		rsyncPodStatus.ContainerElapsedTime = nil
	case !containerStatus.Ready && containerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated.ExitCode != 0:
		// pod has a failure, report last failure reason
		rsyncPodStatus.PodPhase = kapi.PodFailed
		rsyncPodStatus.LogMessage = containerStatus.LastTerminationState.Terminated.Message
		percentProgress := GetProgressPercent(containerStatus.LastTerminationState.Terminated.Message)
		if percentProgress != "" {
			rsyncPodStatus.LastObservedProgressPercent = percentProgress
		}
		transferRate := GetTransferRate(containerStatus.LastTerminationState.Terminated.Message)
		if transferRate != "" {
			rsyncPodStatus.LastObservedTransferRate = transferRate
		}
		exitCode := containerStatus.LastTerminationState.Terminated.ExitCode
		rsyncPodStatus.ExitCode = &exitCode
		rsyncPodStatus.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.LastTerminationState.Terminated.FinishedAt.Sub(containerStatus.LastTerminationState.Terminated.StartedAt.Time).Round(time.Second)}
	case !containerStatus.Ready && podRef.Status.Phase == kapi.PodPending && containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == ContainerCreating:
		rsyncPodStatus.PodPhase = kapi.PodPending
		rsyncPodStatus.ContainerElapsedTime = &metav1.Duration{Duration: time.Now().Sub(podRef.CreationTimestamp.Time).Round(time.Second)}
	case podRef.Status.Phase == kapi.PodFailed:
		// Its possible for the succeeded pod to not have containerStatuses at all
		rsyncPodStatus.PodPhase = kapi.PodFailed
		rsyncPodStatus.LogMessage = containerStatus.State.Terminated.Message
		percentProgress := GetProgressPercent(containerStatus.State.Terminated.Message)
		if percentProgress != "" {
			rsyncPodStatus.LastObservedProgressPercent = percentProgress
		}
		transferRate := GetTransferRate(containerStatus.State.Terminated.Message)
		if transferRate != "" {
			rsyncPodStatus.LastObservedTransferRate = transferRate
		}
		exitCode := containerStatus.State.Terminated.ExitCode
		rsyncPodStatus.ExitCode = &exitCode
		rsyncPodStatus.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.State.Terminated.FinishedAt.Sub(containerStatus.State.Terminated.StartedAt.Time).Round(time.Second)}
	case !containerStatus.Ready && containerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated.ExitCode == 0:
		// succeeded dont ever requeue
		rsyncPodStatus.PodPhase = kapi.PodSucceeded
		rsyncPodStatus.LastObservedProgressPercent = "100%"
		exitCode := containerStatus.LastTerminationState.Terminated.ExitCode
		rsyncPodStatus.ExitCode = &exitCode
		rsyncPodStatus.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.LastTerminationState.Terminated.FinishedAt.Sub(containerStatus.LastTerminationState.Terminated.StartedAt.Time).Round(time.Second)}
	case podRef.Status.Phase == kapi.PodSucceeded:
		// Its possible for the succeeded pod to not have containerStatuses at all
		rsyncPodStatus.PodPhase = kapi.PodSucceeded
		rsyncPodStatus.LastObservedProgressPercent = "100%"
		exitCode := containerStatus.State.Terminated.ExitCode
		rsyncPodStatus.ExitCode = &exitCode
		rsyncPodStatus.ContainerElapsedTime = &metav1.Duration{Duration: containerStatus.State.Terminated.FinishedAt.Sub(containerStatus.State.Terminated.StartedAt.Time).Round(time.Second)}
	}
	return &rsyncPodStatus
}

func getPod(client compat.Client, podReference *kapi.ObjectReference) (*kapi.Pod, error) {
	pod := &kapi.Pod{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: podReference.Namespace,
		Name:      podReference.Name,
	}, pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func (r *RsyncPodProgressTask) getAllMatchingRsyncPods() (*kapi.PodList, error) {
	podList := kapi.PodList{}
	err := r.SrcClient.List(context.TODO(),
		client.InNamespace(r.Owner.Spec.PodNamespace).MatchingLabels(r.Owner.Spec.PodSelector), &podList)
	if err != nil {
		return nil, err
	}
	return &podList, nil
}

func (r *RsyncPodProgressTask) getPodLogs(pod *kapi.Pod, containerName string, tailLines *int64, previous bool) (string, error) {
	config, err := r.Cluster.BuildRestConfig(r.Client)
	if err != nil {
		return "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &kapi.PodLogOptions{
		TailLines: tailLines,
		Previous:  previous,
		Container: containerName,
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

// ProgressStringToValue parses string and returns percentage as a value
func ProgressStringToValue(progressPercentage string) int64 {
	value := int64(0)
	r := regexp.MustCompile(`(\d+)\%`)
	matched := r.FindStringSubmatch(progressPercentage)
	if len(matched) == 2 {
		if v, err := strconv.ParseInt(matched[1], 10, 64); err == nil {
			value = v
		}
	}
	return value
}

// ProgressValueToString parses quantity and returns percentage as a string
func ProgressValueToString(progressPercentage int64) string {
	return fmt.Sprintf("%d%%", progressPercentage)
}

func MaxProgressString(p1 string, p2 string) string {
	return ProgressValueToString(int64(
		math.Max(float64(
			ProgressStringToValue(p1)), float64(
			ProgressStringToValue(p2)))))
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
