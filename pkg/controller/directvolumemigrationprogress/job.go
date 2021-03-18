package directvolumemigrationprogress

import (
	"context"
	"fmt"
	"time"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RsyncContainerName container name in the Rsync Pod
const RsyncContainerName = "rsync-client"

// RsyncClientJobContext defines context for Rsync Job operations
type RsyncClientJobContext struct {
	Client     compat.Client
	Job        *batchv1.Job
	DVMP       *migapi.DirectVolumeMigrationProgress
	RestConfig *rest.Config
}

// EnsureRsyncClientJobStatus updates DVMP status with observed status of Rsync Job
func (ctx *RsyncClientJobContext) EnsureRsyncClientJobStatus() error {
	var err error
	// get all pods
	podList, err := ctx.getAllRsyncClientPods()
	if err != nil {
		return liberr.Wrap(err)
	}
	err = ctx.buildRsyncClientPodsHistory(podList)
	if err != nil {
		return liberr.Wrap(err)
	}
	ctx.ensureCumulativeProgressPercentage()
	// derive Job status
	return nil
}

// getAllRsyncClientPods returns all Rsync client pods associated with the Job
func (ctx *RsyncClientJobContext) getAllRsyncClientPods() (*corev1.PodList, error) {
	if ctx.Job == nil {
		return nil, liberr.Wrap(fmt.Errorf("job cannot be nil"))
	}
	podList := corev1.PodList{}
	labels := map[string]string{
		"job-name": ctx.Job.Name,
	}
	err := ctx.Client.List(context.TODO(), client.InNamespace(ctx.Job.Namespace).MatchingLabels(labels), &podList)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return &podList, nil
}

// getPodLogs given a pod and number of lines to read, returns log lines
func (ctx *RsyncClientJobContext) getPodLogs(pod *corev1.Pod, tailLines *int64) (string, error) {
	clientset, err := kubernetes.NewForConfig(ctx.RestConfig)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &kapi.PodLogOptions{
		TailLines: tailLines,
		Container: RsyncContainerName,
	})
	readCloser, err := req.Stream()
	if err != nil {
		return "", err
	}
	defer readCloser.Close()
	return parseLogs(readCloser)
}

// getRsyncPodStatus builds Rsync Pod Status for an individual pod based on container statuses
// returns reference to pod status and a boolean indicating whether the Pod is in terminal state
func (ctx *RsyncClientJobContext) getRsyncPodStatus(podRef *corev1.Pod) (*migapi.RsyncPodStatus, bool, error) {
	isTerminal := true
	// TODO: confirm container name for Rsync
	hundred := int64(100)
	podStatus := migapi.RsyncPodStatus{
		PodName:           podRef.Name,
		PodPhase:          podRef.Status.Phase,
		CreationTimestamp: podRef.CreationTimestamp,
	}
	var containerStatus *corev1.ContainerStatus
	for i := range podRef.Status.ContainerStatuses {
		cRef := &podRef.Status.ContainerStatuses[i]
		if cRef.Name == RsyncContainerName {
			containerStatus = cRef
		}
	}
	if containerStatus == nil {
		if podRef.Status.Phase == corev1.PodFailed {
			podStatus.PodPhase = corev1.PodFailed
			podStatus.LogMessage = podRef.Status.Message
			podStatus.ContainerElapsedTime = nil
		}
	} else {
		switch {
		case containerStatus.Ready:
			// TODO: ask Alay why not use Pod's phase directly?
			podStatus.PodPhase = corev1.PodRunning
			numberOfLogLines := int64(5)
			logMessage, err := ctx.getPodLogs(podRef, &numberOfLogLines)
			if err != nil {
				return &podStatus, isTerminal, liberr.Wrap(err)
			}
			podStatus.LogMessage = logMessage
			percentProgress := GetProgressPercent(logMessage)
			if percentProgress != "" {
				podStatus.LastObservedProgressPercent = percentProgress
			}
			transferRate := GetTransferRate(logMessage)
			if transferRate != "" {
				podStatus.LastObservedTransferRate = transferRate
			}
			podStatus.ContainerElapsedTime = nil
			isTerminal = false
		case !containerStatus.Ready &&
			podRef.Status.Phase == corev1.PodPending &&
			containerStatus.State.Waiting != nil &&
			containerStatus.State.Waiting.Reason == ContainerCreating:
			// if pod is not in running state after 10 mins of its creation, raise a
			if time.Now().UTC().Sub(podRef.CreationTimestamp.Time.UTC()) > TimeLimit {
				podStatus.PodPhase = corev1.PodPending
				podStatus.LogMessage = fmt.Sprintf("Pod %s/%s is stuck in Pending state for more than 10 mins", podRef.Namespace, podRef.Name)
				podStatus.ContainerElapsedTime = &metav1.Duration{
					Duration: time.Since(podRef.CreationTimestamp.Time).Round(time.Second)}
			}
			isTerminal = false
		case !containerStatus.Ready &&
			containerStatus.LastTerminationState.Terminated != nil &&
			containerStatus.LastTerminationState.Terminated.ExitCode != 0:
			// pod has a failure, report last failure reason
			podStatus.PodPhase = corev1.PodFailed
			podStatus.LogMessage = containerStatus.LastTerminationState.Terminated.Message
			exitCode := containerStatus.LastTerminationState.Terminated.ExitCode
			podStatus.ExitCode = &exitCode
			podStatus.ContainerElapsedTime = &metav1.Duration{
				Duration: containerStatus.LastTerminationState.Terminated.FinishedAt.Sub(containerStatus.LastTerminationState.Terminated.StartedAt.Time).Round(time.Second)}
		case podRef.Status.Phase == corev1.PodFailed:
			// Its possible for the succeeded pod to not have containerStatuses at all
			podStatus.PodPhase = corev1.PodFailed
			podStatus.LogMessage = containerStatus.State.Terminated.Message
			exitCode := containerStatus.State.Terminated.ExitCode
			podStatus.ExitCode = &exitCode
			podStatus.ContainerElapsedTime = &metav1.Duration{
				Duration: containerStatus.State.Terminated.FinishedAt.Sub(containerStatus.State.Terminated.StartedAt.Time).Round(time.Second)}
		case !containerStatus.Ready &&
			containerStatus.LastTerminationState.Terminated != nil &&
			containerStatus.LastTerminationState.Terminated.ExitCode == 0:
			// succeeded dont ever requeue
			podStatus.PodPhase = corev1.PodSucceeded
			podStatus.LastObservedProgressPercent = ProgressPercentageToString(&hundred)
			exitCode := containerStatus.LastTerminationState.Terminated.ExitCode
			podStatus.ExitCode = &exitCode
			podStatus.ContainerElapsedTime = &metav1.Duration{
				Duration: containerStatus.LastTerminationState.Terminated.FinishedAt.Sub(containerStatus.LastTerminationState.Terminated.StartedAt.Time).Round(time.Second)}
		case podRef.Status.Phase == corev1.PodSucceeded:
			// Its possible for the succeeded pod to not have containerStatuses at all
			podStatus.PodPhase = corev1.PodSucceeded
			podStatus.LastObservedProgressPercent = ProgressPercentageToString(&hundred)
			exitCode := containerStatus.State.Terminated.ExitCode
			podStatus.ExitCode = &exitCode
			podStatus.ContainerElapsedTime = &metav1.Duration{
				Duration: containerStatus.State.Terminated.FinishedAt.Sub(containerStatus.State.Terminated.StartedAt.Time).Round(time.Second)}
		}
	}
	return &podStatus, isTerminal, nil
}

// buildRsyncClientPodsHistory builds history for all Rsync pods
func (ctx *RsyncClientJobContext) buildRsyncClientPodsHistory(podList *corev1.PodList) error {
	for i := range podList.Items {
		podRef := &podList.Items[i]
		if !ctx.DVMP.Status.RsyncPodExistsInHistory(podRef.Name) {
			podStatus, isTerminal, err := ctx.getRsyncPodStatus(podRef)
			if err != nil {
				return liberr.Wrap(err)
			}
			if isTerminal {
				ctx.DVMP.Status.RsyncPodStatuses = append(ctx.DVMP.Status.RsyncPodStatuses, *podStatus)
			}
		}
	}
	return nil
}

// ensureCumulativeProgressPercentage ensures total progress percentage is up-to-date
func (ctx *RsyncClientJobContext) ensureCumulativeProgressPercentage() {
	totalPercentage := int64(0)
	for _, podStatus := range ctx.DVMP.Status.RsyncPodStatuses {
		percentage := ProgressPercentageToQuantity(podStatus.LastObservedProgressPercent)
		if percentage != nil {
			totalPercentage += *percentage
		}
	}
	ctx.DVMP.Status.TotalProgressPercentage = ProgressPercentageToString(&totalPercentage)
}
