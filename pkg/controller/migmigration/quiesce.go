package migmigration

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	liberr "github.com/konveyor/controller/pkg/error"
	ocappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Quiesce applications on source cluster
func (t *Task) quiesceApplications() error {
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceCronJobs(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceDeploymentConfigs(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceDeployments(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceStatefulSets(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceReplicaSets(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceDaemonSets(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceJobs(client)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

func (t *Task) unQuiesceSrcApplications() error {
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	t.Log.V(2).Info("Unquiescing applications on source cluster.")
	err = t.unQuiesceApplications(srcClient, t.sourceNamespaces())
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (t *Task) unQuiesceDestApplications() error {
	destClient, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	t.Log.V(2).Info("Unquiescing applications on destination cluster.")
	err = t.unQuiesceApplications(destClient, t.destinationNamespaces())
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

// Unquiesce applications using client and namespace list given
func (t *Task) unQuiesceApplications(client k8sclient.Client, namespaces []string) error {
	err := t.unQuiesceCronJobs(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.unQuiesceDeploymentConfigs(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.unQuiesceDeployments(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.unQuiesceStatefulSets(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.unQuiesceReplicaSets(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.unQuiesceDaemonSets(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.unQuiesceJobs(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Scales down DeploymentConfig on source cluster
func (t *Task) quiesceDeploymentConfigs(client k8sclient.Client) error {
	for _, ns := range t.sourceNamespaces() {
		list := ocappsv1.DeploymentConfigList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, dc := range list.Items {
			if dc.Annotations == nil {
				dc.Annotations = make(map[string]string)
			}
			if dc.Spec.Replicas == 0 {
				continue
			}
			dc.Annotations[ReplicasAnnotation] = strconv.FormatInt(int64(dc.Spec.Replicas), 10)
			t.Log.V(2).Info(fmt.Sprintf("Quiescing DeploymentConfig [%v/%v]. "+
				"Changing .Spec.Replicas from %v -> 0. "+
				"Annotating with %v: %v",
				dc.Namespace, dc.Name,
				dc.Spec.Replicas,
				ReplicasAnnotation, dc.Spec.Replicas))
			dc.Spec.Replicas = 0
			err = client.Update(context.TODO(), &dc)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales DeploymentConfig back up on source cluster
func (t *Task) unQuiesceDeploymentConfigs(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := ocappsv1.DeploymentConfigList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, dc := range list.Items {
			if dc.Annotations == nil {
				continue
			}
			replicas, exist := dc.Annotations[ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(dc.Annotations, ReplicasAnnotation)
			currentReplicas := dc.Spec.Replicas
			// Only set replica count if currently 0
			if dc.Spec.Replicas == 0 {
				dc.Spec.Replicas = int32(number)
			}
			t.Log.V(2).Info(fmt.Sprintf("Unquiescing DeploymentConfig [%v/%v]. "+
				"Changing .Spec.Replicas from %v -> %v. "+
				"Removing Annotation %v",
				dc.Namespace, dc.Name,
				currentReplicas, number,
				ReplicasAnnotation))
			err = client.Update(context.TODO(), &dc)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales down all Deployments
func (t *Task) quiesceDeployments(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, deployment := range list.Items {
			if deployment.Annotations == nil {
				deployment.Annotations = make(map[string]string)
			}
			if *deployment.Spec.Replicas == zero {
				continue
			}
			t.Log.V(2).Info(fmt.Sprintf("Quiescing Deployment [%v/%v] "+
				"Changing spec.Replicas from %v -> 0. "+
				"Annotating with %v: %v",
				deployment.Namespace, deployment.Name,
				deployment.Spec.Replicas,
				ReplicasAnnotation, deployment.Spec.Replicas))
			deployment.Annotations[ReplicasAnnotation] = strconv.FormatInt(int64(*deployment.Spec.Replicas), 10)
			deployment.Spec.Replicas = &zero
			err = client.Update(context.TODO(), &deployment)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales all Deployments back up
func (t *Task) unQuiesceDeployments(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := appsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, deployment := range list.Items {
			if deployment.Annotations == nil {
				deployment.Annotations = make(map[string]string)
			}
			replicas, exist := deployment.Annotations[ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(deployment.Annotations, ReplicasAnnotation)
			restoredReplicas := int32(number)
			currentReplicas := deployment.Spec.Replicas
			// Only change replica count if currently == 0
			if *deployment.Spec.Replicas == 0 {
				deployment.Spec.Replicas = &restoredReplicas
			}
			t.Log.V(2).Info(fmt.Sprintf("Unquiescing Deployment [%v/%v] "+
				"Changing spec.Replicas from %v -> %v. "+
				"Removing Annotation %v",
				deployment.Namespace, deployment.Name,
				currentReplicas, restoredReplicas,
				ReplicasAnnotation))
			err = client.Update(context.TODO(), &deployment)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales down all StatefulSets.
func (t *Task) quiesceStatefulSets(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.StatefulSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			t.Log.V(2).Info(fmt.Sprintf("Quiescing StatefulSet [%v/%v] "+
				"Changing spec.Replicas from %v -> %v. "+
				"Annotating with %v: %v",
				set.Namespace, set.Name,
				set.Spec.Replicas, zero,
				ReplicasAnnotation, set.Spec.Replicas))
			if set.Annotations == nil {
				set.Annotations = make(map[string]string)
			}
			if *set.Spec.Replicas == zero {
				continue
			}
			set.Annotations[ReplicasAnnotation] = strconv.FormatInt(int64(*set.Spec.Replicas), 10)
			set.Spec.Replicas = &zero
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Scales all StatefulSets back up
func (t *Task) unQuiesceStatefulSets(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := appsv1.StatefulSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if set.Annotations == nil {
				continue
			}
			replicas, exist := set.Annotations[ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(set.Annotations, ReplicasAnnotation)
			restoredReplicas := int32(number)

			t.Log.V(2).Info(fmt.Sprintf("Unquiescing StatefulSet [%v/%v]. "+
				"Changing spec.Replicas from %v -> %v. "+
				"Removing Annotation %v.",
				set.Namespace, set.Name,
				set.Spec.Replicas, replicas,
				ReplicasAnnotation))

			// Only change replica count if currently == 0
			if *set.Spec.Replicas == 0 {
				set.Spec.Replicas = &restoredReplicas
			}
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Scales down all ReplicaSets.
func (t *Task) quiesceReplicaSets(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.ReplicaSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if len(set.OwnerReferences) > 0 {
				t.Log.Info(fmt.Sprintf("Quiesce skipping ReplicaSet [%v/%v], has OwnerReferences",
					set.Namespace, set.Name))
				continue
			}
			if set.Annotations == nil {
				set.Annotations = make(map[string]string)
			}
			if *set.Spec.Replicas == zero {
				continue
			}
			set.Annotations[ReplicasAnnotation] = strconv.FormatInt(int64(*set.Spec.Replicas), 10)
			t.Log.V(2).Info(fmt.Sprintf("Quiescing ReplicaSet [%v/%v]. "+
				"Changing spec.Replicas from %v -> %v. "+
				"Setting Annotation %v: %v",
				set.Namespace, set.Name,
				set.Spec.Replicas, zero,
				ReplicasAnnotation, set.Spec.Replicas))
			set.Spec.Replicas = &zero
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Scales all ReplicaSets back up
func (t *Task) unQuiesceReplicaSets(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := appsv1.ReplicaSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if len(set.OwnerReferences) > 0 {
				t.Log.Info("Unquiesce skipping ReplicaSet, has OwnerReferences", "name", set.Name)
				continue
			}
			if set.Annotations == nil {
				continue
			}
			replicas, exist := set.Annotations[ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(set.Annotations, ReplicasAnnotation)
			restoredReplicas := int32(number)
			// Only change replica count if currently == 0
			if *set.Spec.Replicas == 0 {
				set.Spec.Replicas = &restoredReplicas
			}
			t.Log.V(2).Info(fmt.Sprintf("Unquiescing ReplicaSet [%v/%v]. "+
				"Changing spec.Replicas from %v -> %v. "+
				"Removing Annotation %v",
				set.Namespace, set.Name, 0, restoredReplicas, ReplicasAnnotation))
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Scales down all DaemonSets.
func (t *Task) quiesceDaemonSets(client k8sclient.Client) error {
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.DaemonSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if set.Annotations == nil {
				set.Annotations = make(map[string]string)
			}
			if set.Spec.Template.Spec.NodeSelector == nil {
				set.Spec.Template.Spec.NodeSelector = map[string]string{}
			} else if _, exist := set.Spec.Template.Spec.NodeSelector[QuiesceNodeSelector]; exist {
				continue
			}
			selector, err := json.Marshal(set.Spec.Template.Spec.NodeSelector)
			if err != nil {
				return liberr.Wrap(err)
			}
			set.Annotations[NodeSelectorAnnotation] = string(selector)
			set.Spec.Template.Spec.NodeSelector[QuiesceNodeSelector] = "true"
			t.Log.V(2).Info(fmt.Sprintf("Quiescing DaemonSet [%v/%v]. "+
				"Changing Spec.Template.Spec.NodeSelector to %v:true. "+
				"Setting annotation %v: %v",
				set.Namespace, set.Name, QuiesceNodeSelector,
				NodeSelectorAnnotation, string(selector)))
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Scales all DaemonSets back up
func (t *Task) unQuiesceDaemonSets(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := appsv1.DaemonSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if set.Annotations == nil {
				continue
			}
			selector, exist := set.Annotations[NodeSelectorAnnotation]
			if !exist {
				continue
			}
			nodeSelector := map[string]string{}
			err := json.Unmarshal([]byte(selector), &nodeSelector)
			if err != nil {
				return liberr.Wrap(err)
			}
			// Only change node selector if set to our quiesce nodeselector
			_, isQuiesced := set.Spec.Template.Spec.NodeSelector[QuiesceNodeSelector]
			if !isQuiesced {
				continue
			}
			delete(set.Annotations, NodeSelectorAnnotation)
			set.Spec.Template.Spec.NodeSelector = nodeSelector
			t.Log.V(2).Info(fmt.Sprintf("Unquiescing DaemonSet [%v/%v]. "+
				"Setting Spec.Template.Spec.NodeSelector=%v. "+
				"Removing Annotation %v.",
				set.Namespace, set.Name, nodeSelector, NodeSelectorAnnotation))
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Suspends all CronJobs
func (t *Task) quiesceCronJobs(client k8sclient.Client) error {
	for _, ns := range t.sourceNamespaces() {
		list := batchv1beta.CronJobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, &list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, r := range list.Items {
			if r.Annotations == nil {
				r.Annotations = make(map[string]string)
			}
			if r.Spec.Suspend == pointer.BoolPtr(true) {
				continue
			}
			r.Annotations[SuspendAnnotation] = "true"
			r.Spec.Suspend = pointer.BoolPtr(true)
			t.Log.V(2).Info(fmt.Sprintf("Quiescing Job [%v/%v]. "+
				"Setting Spec.Suspend=true. "+
				"Setting Annotation %v: true",
				r.Namespace, r.Name, SuspendAnnotation))
			err = client.Update(context.TODO(), &r)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Undo quiescence on all CronJobs
func (t *Task) unQuiesceCronJobs(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := batchv1beta.CronJobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, &list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, r := range list.Items {
			if r.Annotations == nil {
				continue
			}
			// Only unsuspend if our suspend annotation is present
			if _, exist := r.Annotations[SuspendAnnotation]; !exist {
				continue
			}
			delete(r.Annotations, SuspendAnnotation)
			r.Spec.Suspend = pointer.BoolPtr(false)
			t.Log.V(2).Info(fmt.Sprintf("Unquiescing Cron Job [%v/%v]."+
				"Setting Spec.Suspend=false",
				r.Namespace, r.Name))
			err = client.Update(context.TODO(), &r)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales down all Jobs
func (t *Task) quiesceJobs(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := batchv1.JobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, job := range list.Items {
			if job.Annotations == nil {
				job.Annotations = make(map[string]string)
			}
			if job.Spec.Parallelism == &zero {
				continue
			}
			job.Annotations[ReplicasAnnotation] = strconv.FormatInt(int64(*job.Spec.Parallelism), 10)
			job.Spec.Parallelism = &zero
			t.Log.V(2).Info(fmt.Sprintf("Quiescing Job [%v/%v]. "+
				"Setting Spec.Parallelism=0. "+
				"Annotating with %v: %v",
				job.Namespace, job.Name, ReplicasAnnotation, job.Spec.Parallelism))
			err = client.Update(context.TODO(), &job)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales all Jobs back up
func (t *Task) unQuiesceJobs(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := batchv1.JobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, job := range list.Items {
			if job.Annotations == nil {
				continue
			}
			replicas, exist := job.Annotations[ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(job.Annotations, ReplicasAnnotation)
			parallelReplicas := int32(number)
			// Only change parallelism if currently == 0
			if *job.Spec.Parallelism == 0 {
				job.Spec.Parallelism = &parallelReplicas
			}
			t.Log.V(2).Info(fmt.Sprintf("Unquiescing Job [%v/%v]. "+
				"Setting Spec.Parallelism=%v"+
				"Removing Annotation %v",
				job.Namespace, job.Name, parallelReplicas, ReplicasAnnotation))
			err = client.Update(context.TODO(), &job)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Ensure scaled down pods have terminated.
// Returns: `true` when all pods terminated.
func (t *Task) ensureQuiescedPodsTerminated() (bool, error) {
	kinds := map[string]bool{
		"ReplicationController": true,
		"StatefulSet":           true,
		"ReplicaSet":            true,
		"DaemonSet":             true,
		"Job":                   true,
	}
	skippedPhases := map[v1.PodPhase]bool{
		v1.PodSucceeded: true,
		v1.PodFailed:    true,
		v1.PodUnknown:   true,
	}
	client, err := t.getSourceClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	for _, ns := range t.sourceNamespaces() {
		list := v1.PodList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return false, liberr.Wrap(err)
		}
		for _, pod := range list.Items {
			if pod.Annotations == nil {
				pod.Annotations = make(map[string]string)
			}
			if _, found := skippedPhases[pod.Status.Phase]; found {
				continue
			}
			for _, ref := range pod.OwnerReferences {
				if _, found := kinds[ref.Kind]; found {
					t.Log.V(2).Info(fmt.Sprintf("Found quiesced Pod [%v/%v] on source cluster"+
						" that has not yet terminated. Waiting.",
						pod.Namespace, pod.Name))
					return false, nil
				}
			}
		}
	}

	return true, nil
}
