package migmigration

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	ocappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	virtv1 "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	vmSubresourceURLFmt = "/apis/subresources.kubevirt.io/%s/namespaces/%s/virtualmachines/%s/%s"
)

func (t *Task) quiesceSourceApplications() error {
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	restConfig, err := t.getSourceRestConfig()
	if err != nil {
		return liberr.Wrap(err)
	}
	return t.quiesceApplications(client, restConfig, t.sourceNamespaces())
}

func (t *Task) quiesceDestinationApplications() error {
	client, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	restConfig, err := t.getDestinationRestConfig()
	if err != nil {
		return liberr.Wrap(err)
	}
	return t.quiesceApplications(client, restConfig, t.destinationNamespaces())
}

// Quiesce applications on source cluster
func (t *Task) quiesceApplications(client compat.Client, restConfig *rest.Config, namespaces []string) error {
	err := t.quiesceCronJobs(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceDeploymentConfigs(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceDeployments(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceStatefulSets(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceReplicaSets(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceDaemonSets(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = t.quiesceJobs(client, namespaces)
	if err != nil {
		return liberr.Wrap(err)
	}
	if !t.PlanResources.MigPlan.LiveMigrationChecked() {
		restClient, err := t.createRestClient(restConfig)
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.quiesceVirtualMachines(client, restClient, namespaces)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

func (t *Task) unQuiesceSrcApplications() error {
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	restConfig, err := t.getSourceRestConfig()
	if err != nil {
		return liberr.Wrap(err)
	}
	t.Log.V(3).Info("Unquiescing applications on source cluster.")
	err = t.unQuiesceApplications(srcClient, restConfig, t.sourceNamespaces())
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
	restConfig, err := t.getDestinationRestConfig()
	if err != nil {
		return liberr.Wrap(err)
	}
	t.Log.V(3).Info("Unquiescing applications on destination cluster.")
	err = t.unQuiesceApplications(destClient, restConfig, t.destinationNamespaces())
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

// Unquiesce applications using client and namespace list given
func (t *Task) unQuiesceApplications(client compat.Client, restConfig *rest.Config, namespaces []string) error {
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
	if !t.PlanResources.MigPlan.LiveMigrationChecked() {
		restClient, err := t.createRestClient(restConfig)
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.unQuiesceVirtualMachines(client, restClient, namespaces)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

// Scales down DeploymentConfig on source cluster
func (t *Task) quiesceDeploymentConfigs(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := ocappsv1.DeploymentConfigList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
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
			dc.Annotations[migapi.ReplicasAnnotation] = strconv.FormatInt(int64(dc.Spec.Replicas), 10)
			dc.Annotations[migapi.PausedAnnotation] = strconv.FormatBool(dc.Spec.Paused)
			t.Log.Info(fmt.Sprintf("Quiescing DeploymentConfig. "+
				"Changing .Spec.Replicas from [%v->0]. "+
				"Annotating with [%v: %v]",
				dc.Spec.Replicas,
				migapi.ReplicasAnnotation, dc.Spec.Replicas),
				"deploymentConfig", path.Join(dc.Namespace, dc.Name))
			dc.Spec.Replicas = 0
			dc.Spec.Paused = false
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
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, dc := range list.Items {
			if dc.Annotations == nil {
				continue
			}
			replicas, exist := dc.Annotations[migapi.ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(dc.Annotations, migapi.ReplicasAnnotation)
			currentReplicas := dc.Spec.Replicas
			// Only set replica count if currently 0
			if dc.Spec.Replicas == 0 {
				dc.Spec.Replicas = int32(number)
			}
			t.Log.Info(fmt.Sprintf("Unquiescing DeploymentConfig. "+
				"Changing .Spec.Replicas from [%v->%v]. "+
				"Removing Annotation [%v]",
				currentReplicas, number,
				migapi.ReplicasAnnotation),
				"deploymentConfig", path.Join(dc.Namespace, dc.Name))
			err = client.Update(context.TODO(), &dc)
			if err != nil {
				return liberr.Wrap(err)
			}

			// For Deployment Configs we have to set paused separately or pods wont launch
			// We have to get the updated resource otherwise we just produce a conflict
			ref := types.NamespacedName{Name: dc.Name, Namespace: dc.Namespace}
			err = client.Get(context.TODO(), ref, &dc)
			if err != nil {
				return liberr.Wrap(err)
			}
			if val, exists := dc.Annotations[migapi.PausedAnnotation]; exists {
				dc.Spec.Paused, err = strconv.ParseBool(val)
				if err != nil {
					return liberr.Wrap(err)
				}
				delete(dc.Annotations, migapi.PausedAnnotation)
			}
			err = client.Update(context.TODO(), &dc)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales down all Deployments
func (t *Task) quiesceDeployments(client k8sclient.Client, namespaces []string) error {
	zero := int32(0)
	for _, ns := range namespaces {
		list := appsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
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
			t.Log.Info(fmt.Sprintf("Quiescing Deployment. "+
				"Changing spec.Replicas from [%v->0]. "+
				"Annotating with [%v: %v]",
				deployment.Spec.Replicas,
				migapi.ReplicasAnnotation, deployment.Spec.Replicas),
				"deployment", path.Join(deployment.Namespace, deployment.Name))
			deployment.Annotations[migapi.ReplicasAnnotation] = strconv.FormatInt(int64(*deployment.Spec.Replicas), 10)
			deployment.Annotations[migapi.PausedAnnotation] = strconv.FormatBool(deployment.Spec.Paused)
			deployment.Spec.Replicas = &zero
			deployment.Spec.Paused = false
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
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, deployment := range list.Items {
			if deployment.Annotations == nil {
				deployment.Annotations = make(map[string]string)
			}
			replicas, exist := deployment.Annotations[migapi.ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			if val, exists := deployment.Annotations[migapi.PausedAnnotation]; exists {
				deployment.Spec.Paused, err = strconv.ParseBool(val)
				if err != nil {
					return liberr.Wrap(err)
				}
				delete(deployment.Annotations, migapi.PausedAnnotation)
			}
			delete(deployment.Annotations, migapi.ReplicasAnnotation)
			restoredReplicas := int32(number)
			currentReplicas := deployment.Spec.Replicas
			// Only change replica count if currently == 0
			if *deployment.Spec.Replicas == 0 {
				deployment.Spec.Replicas = &restoredReplicas
			}
			t.Log.Info(fmt.Sprintf("Unquiescing Deployment. "+
				"Changing Spec.Replicas from [%v->%v]. "+
				"Removing Annotation [%v]",
				currentReplicas, restoredReplicas,
				migapi.ReplicasAnnotation),
				"deployment", path.Join(deployment.Namespace, deployment.Name))
			err = client.Update(context.TODO(), &deployment)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Scales down all StatefulSets.
func (t *Task) quiesceStatefulSets(client k8sclient.Client, namespaces []string) error {
	zero := int32(0)
	for _, ns := range namespaces {
		list := appsv1.StatefulSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			t.Log.Info(fmt.Sprintf("Quiescing StatefulSet. "+
				"Changing Spec.Replicas from [%v->%v]. "+
				"Annotating with [%v: %v]",
				set.Spec.Replicas, zero,
				migapi.ReplicasAnnotation, set.Spec.Replicas),
				"statefulSet", path.Join(set.Namespace, set.Name))
			if set.Annotations == nil {
				set.Annotations = make(map[string]string)
			}
			if *set.Spec.Replicas == zero {
				continue
			}
			set.Annotations[migapi.ReplicasAnnotation] = strconv.FormatInt(int64(*set.Spec.Replicas), 10)
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
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if set.Annotations == nil {
				continue
			}
			replicas, exist := set.Annotations[migapi.ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(set.Annotations, migapi.ReplicasAnnotation)
			restoredReplicas := int32(number)

			t.Log.Info(fmt.Sprintf("Unquiescing StatefulSet. "+
				"Changing Spec.Replicas from [%v->%v]. "+
				"Removing Annotation [%v].",
				set.Spec.Replicas, replicas,
				migapi.ReplicasAnnotation),
				"statefulSet", path.Join(set.Namespace, set.Name))

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
func (t *Task) quiesceReplicaSets(client k8sclient.Client, namespaces []string) error {
	zero := int32(0)
	for _, ns := range namespaces {
		list := appsv1.ReplicaSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if len(set.OwnerReferences) > 0 {
				t.Log.Info("Quiesce skipping ReplicaSet, has OwnerReferences",
					"replicaSet", path.Join(set.Namespace, set.Name))
				continue
			}
			if set.Annotations == nil {
				set.Annotations = make(map[string]string)
			}
			if *set.Spec.Replicas == zero {
				continue
			}
			set.Annotations[migapi.ReplicasAnnotation] = strconv.FormatInt(int64(*set.Spec.Replicas), 10)
			t.Log.Info(fmt.Sprintf("Quiescing ReplicaSet. "+
				"Changing Spec.Replicas from [%v->%v]. "+
				"Setting Annotation [%v: %v]",
				set.Spec.Replicas, zero,
				migapi.ReplicasAnnotation, set.Spec.Replicas),
				"replicaSet", path.Join(set.Namespace, set.Name))
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
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if len(set.OwnerReferences) > 0 {
				t.Log.Info("Unquiesce skipping ReplicaSet, has OwnerReferences",
					"replicaSet", path.Join(set.Namespace, set.Name))
				continue
			}
			if set.Annotations == nil {
				continue
			}
			replicas, exist := set.Annotations[migapi.ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(set.Annotations, migapi.ReplicasAnnotation)
			restoredReplicas := int32(number)
			// Only change replica count if currently == 0
			if *set.Spec.Replicas == 0 {
				set.Spec.Replicas = &restoredReplicas
			}
			t.Log.Info(fmt.Sprintf("Unquiescing ReplicaSet. "+
				"Changing Spec.Replicas from [%v->%v]. "+
				"Removing Annotation [%v]",
				0, restoredReplicas, migapi.ReplicasAnnotation),
				"replicaSet", path.Join(set.Namespace, set.Name))
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Scales down all DaemonSets.
func (t *Task) quiesceDaemonSets(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := appsv1.DaemonSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if set.Annotations == nil {
				set.Annotations = make(map[string]string)
			}
			if set.Spec.Template.Spec.NodeSelector == nil {
				set.Spec.Template.Spec.NodeSelector = map[string]string{}
			} else if _, exist := set.Spec.Template.Spec.NodeSelector[migapi.QuiesceNodeSelector]; exist {
				continue
			}
			selector, err := json.Marshal(set.Spec.Template.Spec.NodeSelector)
			if err != nil {
				return liberr.Wrap(err)
			}
			set.Annotations[migapi.NodeSelectorAnnotation] = string(selector)
			set.Spec.Template.Spec.NodeSelector[migapi.QuiesceNodeSelector] = "true"
			t.Log.Info(fmt.Sprintf("Quiescing DaemonSet. "+
				"Changing [Spec.Template.Spec.NodeSelector=%v:true]. "+
				"Setting annotation [%v: %v]",
				migapi.QuiesceNodeSelector, migapi.NodeSelectorAnnotation, string(selector)),
				"daemonSet", path.Join(set.Namespace, set.Name))
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
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, set := range list.Items {
			if set.Annotations == nil {
				continue
			}
			selector, exist := set.Annotations[migapi.NodeSelectorAnnotation]
			if !exist {
				continue
			}
			nodeSelector := map[string]string{}
			err := json.Unmarshal([]byte(selector), &nodeSelector)
			if err != nil {
				return liberr.Wrap(err)
			}
			// Only change node selector if set to our quiesce nodeselector
			_, isQuiesced := set.Spec.Template.Spec.NodeSelector[migapi.QuiesceNodeSelector]
			if !isQuiesced {
				continue
			}
			delete(set.Annotations, migapi.NodeSelectorAnnotation)
			set.Spec.Template.Spec.NodeSelector = nodeSelector
			t.Log.Info(fmt.Sprintf("Unquiescing DaemonSet. "+
				"Setting [Spec.Template.Spec.NodeSelector=%v]. "+
				"Removing Annotation [%v].",
				nodeSelector, migapi.NodeSelectorAnnotation),
				"daemonSet", path.Join(set.Namespace, set.Name))
			err = client.Update(context.TODO(), &set)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// Suspends all CronJobs
func (t *Task) quiesceCronJobs(client compat.Client, namespaces []string) error {
	for _, ns := range namespaces {

		if client.MinorVersion() < 21 {
			list := batchv1beta.CronJobList{}
			options := k8sclient.InNamespace(ns)
			err := client.List(context.TODO(), &list, options)
			if err != nil {
				return liberr.Wrap(err)
			}
			for _, r := range list.Items {
				if r.Annotations == nil {
					r.Annotations = make(map[string]string)
				}
				if r.Spec.Suspend == ptr.To[bool](true) {
					continue
				}
				r.Annotations[migapi.SuspendAnnotation] = "true"
				r.Spec.Suspend = ptr.To[bool](true)
				t.Log.Info(fmt.Sprintf("Quiescing Job. "+
					"Setting [Spec.Suspend=true]. "+
					"Setting Annotation [%v]: true",
					migapi.SuspendAnnotation),
					"job", path.Join(r.Namespace, r.Name))
				err = client.Update(context.TODO(), &r)
				if err != nil {
					return liberr.Wrap(err)
				}
			}

		} else {
			list := batchv1.CronJobList{}
			options := k8sclient.InNamespace(ns)
			err := client.List(context.TODO(), &list, options)
			if err != nil {
				return liberr.Wrap(err)
			}
			for _, r := range list.Items {
				if r.Annotations == nil {
					r.Annotations = make(map[string]string)
				}
				if r.Spec.Suspend == ptr.To[bool](true) {
					continue
				}
				r.Annotations[migapi.SuspendAnnotation] = "true"
				r.Spec.Suspend = ptr.To[bool](true)
				t.Log.Info(fmt.Sprintf("Quiescing Job. "+
					"Setting [Spec.Suspend=true]. "+
					"Setting Annotation [%v]: true",
					migapi.SuspendAnnotation),
					"job", path.Join(r.Namespace, r.Name))
				err = client.Update(context.TODO(), &r)
				if err != nil {
					return liberr.Wrap(err)
				}
			}

		}
	}

	return nil
}

// Undo quiescence on all CronJobs
func (t *Task) unQuiesceCronJobs(client compat.Client, namespaces []string) error {
	for _, ns := range namespaces {
		if client.MinorVersion() < 21 {
			list := batchv1beta.CronJobList{}
			options := k8sclient.InNamespace(ns)
			err := client.List(context.TODO(), &list, options)
			if err != nil {
				return liberr.Wrap(err)
			}
			for _, r := range list.Items {
				if r.Annotations == nil {
					continue
				}
				// Only unsuspend if our suspend annotation is present
				if _, exist := r.Annotations[migapi.SuspendAnnotation]; !exist {
					continue
				}
				delete(r.Annotations, migapi.SuspendAnnotation)
				r.Spec.Suspend = ptr.To[bool](false)
				t.Log.Info("Unquiescing Cron Job. Setting [Spec.Suspend=false]",
					"cronJob", path.Join(r.Namespace, r.Name))
				err = client.Update(context.TODO(), &r)
				if err != nil {
					return liberr.Wrap(err)
				}
			}

		} else {
			list := batchv1.CronJobList{}
			options := k8sclient.InNamespace(ns)
			err := client.List(context.TODO(), &list, options)
			if err != nil {
				return liberr.Wrap(err)
			}
			for _, r := range list.Items {
				if r.Annotations == nil {
					continue
				}
				// Only unsuspend if our suspend annotation is present
				if _, exist := r.Annotations[migapi.SuspendAnnotation]; !exist {
					continue
				}
				delete(r.Annotations, migapi.SuspendAnnotation)
				r.Spec.Suspend = ptr.To[bool](false)
				t.Log.Info("Unquiescing Cron Job. Setting [Spec.Suspend=false]",
					"cronJob", path.Join(r.Namespace, r.Name))
				err = client.Update(context.TODO(), &r)
				if err != nil {
					return liberr.Wrap(err)
				}
			}

		}
	}

	return nil
}

// Scales down all Jobs
func (t *Task) quiesceJobs(client k8sclient.Client, namespaces []string) error {
	for _, ns := range namespaces {
		list := batchv1.JobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, job := range list.Items {
			if job.Annotations == nil {
				job.Annotations = make(map[string]string)
			}
			if job.Spec.Parallelism == ptr.To[int32](0) {
				continue
			}
			job.Annotations[migapi.ReplicasAnnotation] = strconv.FormatInt(int64(*job.Spec.Parallelism), 10)
			job.Spec.Parallelism = ptr.To[int32](0)
			t.Log.Info(fmt.Sprintf("Quiescing Job. "+
				"Setting [Spec.Parallelism=0]. "+
				"Annotating with [%v: %v]",
				migapi.ReplicasAnnotation, job.Spec.Parallelism),
				"job", path.Join(job.Namespace, job.Name))
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
			&list,
			options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, job := range list.Items {
			if job.Annotations == nil {
				continue
			}
			replicas, exist := job.Annotations[migapi.ReplicasAnnotation]
			if !exist {
				continue
			}
			number, err := strconv.Atoi(replicas)
			if err != nil {
				return liberr.Wrap(err)
			}
			delete(job.Annotations, migapi.ReplicasAnnotation)
			parallelReplicas := int32(number)
			// Only change parallelism if currently == 0
			if *job.Spec.Parallelism == 0 {
				job.Spec.Parallelism = &parallelReplicas
			}
			t.Log.Info(fmt.Sprintf("Unquiescing Job. "+
				"Setting [Spec.Parallelism=%v]"+
				"Removing Annotation [%v]",
				parallelReplicas, migapi.ReplicasAnnotation),
				"job", path.Join(job.Namespace, job.Name))
			err = client.Update(context.TODO(), &job)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

func (t *Task) createRestClient(restConfig *rest.Config) (rest.Interface, error) {
	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	restClient, err := apiutil.RESTClientForGVK(
		virtv1.VirtualMachineGroupVersionKind,
		false,
		restConfig,
		serializer.NewCodecFactory(t.Scheme),
		httpClient)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return restClient, nil
}

// Scales down all Virtual Machines
func (t *Task) quiesceVirtualMachines(client k8sclient.Client, restClient rest.Interface, namespaces []string) error {
	for _, ns := range namespaces {
		list := virtv1.VirtualMachineList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			return liberr.Wrap(err)
		}
		for _, vm := range list.Items {
			if !isVMActive(&vm, client) {
				continue
			}
			if vm.Annotations == nil {
				vm.Annotations = make(map[string]string)
			}
			if vm.Spec.RunStrategy != nil {
				vm.Annotations[migapi.RunStrategyAnnotation] = string(*vm.Spec.RunStrategy)
			}
			vm.Annotations[migapi.StartVMAnnotation] = "true"
			err = client.Update(context.TODO(), &vm)
			if err != nil {
				return liberr.Wrap(err)
			}
			t.Log.Info("Stopping Virtual Machine",
				"vm", path.Join(vm.Namespace, vm.Name))

			stopOptions := &virtv1.StopOptions{
				// 10 minutes grace period
				GracePeriod: ptr.To[int64](600),
			}
			optsJson, err := json.Marshal(stopOptions)
			if err != nil {
				return liberr.Wrap(err)
			}
			uri := fmt.Sprintf(vmSubresourceURLFmt, virtv1.ApiStorageVersion, vm.Namespace, vm.Name, "stop")
			err = restClient.Put().AbsPath(uri).Body(optsJson).Do(context.Background()).Error()
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// Start all Virtual Machines back up
func (t *Task) unQuiesceVirtualMachines(client k8sclient.Client, restClient rest.Interface, namespaces []string) error {
	for _, ns := range namespaces {
		list := virtv1.VirtualMachineList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			return liberr.Wrap(err)
		}
		for _, vm := range list.Items {
			if isVMActive(&vm, client) || !shouldStartVM(&vm) {
				continue
			}
			if err := t.startVM(&vm, client, restClient); err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

func isVMActive(vm *virtv1.VirtualMachine, client k8sclient.Client) bool {
	// A VM is defined active if there is a pod associated with the VM.
	podList := v1.PodList{}
	client.List(context.TODO(), &podList, k8sclient.InNamespace(vm.Namespace))
	for _, pod := range podList.Items {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "VirtualMachineInstance" && owner.Name == vm.Name {
				return true
			}
		}
	}
	return false
}

func shouldStartVM(vm *virtv1.VirtualMachine) bool {
	if vm.Annotations == nil {
		return false
	}
	value, exist := vm.Annotations[migapi.StartVMAnnotation]
	return exist && value == "true"
}

func (t *Task) startVM(vm *virtv1.VirtualMachine, client k8sclient.Client, restClient rest.Interface) error {
	restoreRunStrategy := false
	strategy := ""
	if vm.Annotations != nil {
		runStrategy, exist := vm.Annotations[migapi.RunStrategyAnnotation]
		if exist {
			restoreRunStrategy = true
			strategy = runStrategy
		}
	}
	delete(vm.Annotations, migapi.RunStrategyAnnotation)
	delete(vm.Annotations, migapi.StartVMAnnotation)
	t.Log.Info("Starting Virtual Machine", "vm", path.Join(vm.Namespace, vm.Name))

	if restoreRunStrategy {
		vm.Spec.RunStrategy = (*virtv1.VirtualMachineRunStrategy)(&strategy)
	}

	if err := client.Update(context.Background(), vm); err != nil {
		return liberr.Wrap(err)
	}
	startOptions := &virtv1.StartOptions{}
	optsJson, err := json.Marshal(startOptions)
	if err != nil {
		return liberr.Wrap(err)
	}
	uri := fmt.Sprintf(vmSubresourceURLFmt, virtv1.ApiStorageVersion, vm.Namespace, vm.Name, "start")
	err = restClient.Put().AbsPath(uri).Body(optsJson).Do(context.Background()).Error()
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (t *Task) ensureSourceQuiescedPodsTerminated() (bool, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	return t.ensureQuiescedPodsTerminated(client, t.sourceNamespaces(), "source")
}

func (t *Task) ensureDestinationQuiescedPodsTerminated() (bool, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	return t.ensureQuiescedPodsTerminated(client, t.destinationNamespaces(), "destination")
}

// Ensure scaled down pods have terminated.
// Returns: `true` when all pods terminated.
func (t *Task) ensureQuiescedPodsTerminated(client compat.Client, namespaces []string, clusterType string) (bool, error) {
	kinds := map[string]bool{
		"ReplicationController": true,
		"StatefulSet":           true,
		"ReplicaSet":            true,
		"DaemonSet":             true,
		"Job":                   true,
	}
	if !t.PlanResources.MigPlan.LiveMigrationChecked() {
		kinds["VirtualMachineInstance"] = true
	}
	skippedPhases := map[v1.PodPhase]bool{
		v1.PodSucceeded: true,
		v1.PodFailed:    true,
		v1.PodUnknown:   true,
	}
	for _, ns := range namespaces {
		list := v1.PodList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
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
					t.Log.Info("Found quiesced Pod on "+clusterType+" cluster"+
						" that has not yet terminated. Waiting.",
						"pod", path.Join(pod.Namespace, pod.Name),
						"podPhase", pod.Status.Phase)
					return false, nil
				}
			}
		}
	}

	return true, nil
}
