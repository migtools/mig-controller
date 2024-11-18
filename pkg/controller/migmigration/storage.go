package migmigration

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	dvmc "github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	ocappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	virtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// PVCNameMapping is a mapping for source -> destination pvc names
// used for convenience to avoid nested lookups to find migrated PVC names
type pvcNameMapping map[string]string

const (
	statefulSetUpdateAnnotation = "migration.openshift.io/statefulset-update"
	statefulSetTempAnnotation   = "migration.openshift.io/statefulset-temporary"
)

// Add adds a new PVC to mapping
func (p pvcNameMapping) Add(namespace string, srcName string, destName string) {
	if p == nil {
		p = make(pvcNameMapping)
	}
	key := fmt.Sprintf("%s/%s", namespace, srcName)
	p[key] = destName
}

// Get given a source PVC namespace and name, returns associated destination PVC name and ns
func (p pvcNameMapping) Get(namespace string, srcName string) (string, bool) {
	key := fmt.Sprintf("%s/%s", namespace, srcName)
	val, exists := p[key]
	return val, exists
}

// ExistsAsValue given a PVC name, tells whether it exists as a destination name
func (p pvcNameMapping) ExistsAsValue(destName string) bool {
	for _, v := range p {
		if destName == v {
			return true
		}
	}
	return false
}

// swapPVCReferences for storage conversion migrations, this method
// swaps the existing PVC references on workload resources with the
// new pvcs created during storage migration
// works on following workload resources:
// - daemonsets
// - deploymentconfigs
// - deployments
// - replicasets
// - statefulsets
// - jobs
// - cronjobs
// - virtual machines
func (t *Task) swapPVCReferences() (reasons []string, err error) {
	client, err := t.getDestinationClient()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	restConfig, err := t.getDestinationRestConfig()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	// build a mapping of source to destination pvc names to avoid nested loops
	mapping := t.getPVCNameMapping()
	// update pvc refs on deployments
	failedDeploymentNames := t.swapDeploymentPVCRefs(client, mapping)
	if len(failedDeploymentNames) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on Deployments [%s]", strings.Join(failedDeploymentNames, ",")))
	}
	// update pvc refs on deploymentconfigs
	failedDeploymentConfigNames := t.swapDeploymentConfigPVCRefs(client, mapping)
	if len(failedDeploymentConfigNames) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on DeploymentConfigs [%s]", strings.Join(failedDeploymentConfigNames, ",")))
	}
	// update pvc refs on replicasets
	failedReplicaSetNames := t.swapReplicaSetsPVCRefs(client, mapping)
	if len(failedReplicaSetNames) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on ReplicaSets [%s]", strings.Join(failedReplicaSetNames, ",")))
	}
	// update pvc refs on daemonsets
	failedDaemonSetNames := t.swapDaemonSetsPVCRefs(client, mapping)
	if len(failedDaemonSetNames) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on DaemonSets [%s]", strings.Join(failedDaemonSetNames, ",")))
	}
	// update pvc refs on jobs
	failedJobNames := t.swapJobsPVCRefs(client, mapping)
	if len(failedJobNames) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on Jobs [%s]", strings.Join(failedJobNames, ",")))
	}
	// update pvc refs on cronjobs
	failedCronJobNames := t.swapCronJobsPVCRefs(client, mapping)
	if len(failedCronJobNames) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on CronJobs [%s]", strings.Join(failedCronJobNames, ",")))
	}
	// update pvc refs on statefulsets
	failedStatefulSetNames := t.swapStatefulSetPVCRefs(client, mapping)
	if len(failedStatefulSetNames) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on StatefulSets [%s]", strings.Join(failedStatefulSetNames, ",")))
	}
	restClient, err := t.createRestClient(restConfig)
	if err != nil {
		t.Log.Error(err, "failed creating rest client")
	}
	failedVirtualMachineSwaps := t.swapVirtualMachinePVCRefs(client, restClient, mapping)
	if len(failedVirtualMachineSwaps) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating PVC references on VirtualMachines [%s]", strings.Join(failedVirtualMachineSwaps, ",")))
	}
	sourceClient, err := t.getSourceClient()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	failedHandleSourceLabels := t.handleSourceLabels(sourceClient, mapping)
	if len(failedHandleSourceLabels) > 0 {
		reasons = append(reasons,
			fmt.Sprintf("Failed updating labels on source PVCs [%s]", strings.Join(failedHandleSourceLabels, ",")))
	}
	return
}

func (t *Task) getPVCNameMapping() pvcNameMapping {
	mapping := make(pvcNameMapping)
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		if pv.Selection.Action == migapi.PvSkipAction {
			// for skipped pvcs, there is no need to switch PVC references
			mapping.Add(pv.PVC.Namespace, pv.PVC.GetSourceName(), pv.PVC.GetSourceName())
		} else if t.rollback() {
			// If this is a rollback migration, the mapping of PVC names should be Destination -> Source
			mapping.Add(pv.PVC.Namespace, pv.PVC.GetTargetName(), pv.PVC.GetSourceName())
		} else {
			mapping.Add(pv.PVC.Namespace, pv.PVC.GetSourceName(), pv.PVC.GetTargetName())
		}
	}
	return mapping
}

func (t *Task) handleSourceLabels(client k8sclient.Client, mapping pvcNameMapping) (failedPVCs []string) {
	if t.rollback() {
		return
	}
	for _, ns := range t.sourceNamespaces() {
		list := v1.PersistentVolumeClaimList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			failedPVCs = append(failedPVCs, fmt.Sprintf("failed listing PVCs in namespace %s", ns))
			continue
		}
		for _, pvc := range list.Items {
			labels := pvc.Labels
			if labels == nil {
				labels = make(map[string]string)
			}
			if mapping.ExistsAsValue(pvc.Name) {
				//Skip target PVCs if they are in the same namespace, ensure the label was not copied
				//from the source PVC
				delete(labels, directvolumemigration.MigrationSourceFor)
			} else {
				// Migration completed successfully, mark PVCs as migrated.
				labels[directvolumemigration.MigrationSourceFor] = string(t.PlanResources.MigPlan.UID)
			}
			pvc.Labels = labels
			if err := client.Update(context.TODO(), &pvc); err != nil {
				failedPVCs = append(failedPVCs, fmt.Sprintf("failed to modify labels on PVC %s/%s", pvc.Namespace, pvc.Name))
			}
		}
	}
	return failedPVCs
}

func (t *Task) swapStatefulSetPVCRefs(client k8sclient.Client, mapping pvcNameMapping) (failedStatefulSets []string) {
	for _, ns := range t.destinationNamespaces() {
		list := appsv1.StatefulSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			t.Log.Error(err, "failed listing statefulsets", "namespace", ns)
			continue
		}
		for i := range list.Items {
			set := &list.Items[i]
			newSet := &appsv1.StatefulSet{}
			if len(set.Name) >= 60 {
				failedStatefulSets = append(failedStatefulSets,
					fmt.Sprintf("%s/%s", set.Namespace, set.Name))
				continue
			}
			set.ResourceVersion = ""
			set.ManagedFields = nil
			set.SelfLink = ""
			newSet.Name = fmt.Sprintf("%s-%s", set.Name, migapi.StorageConversionPVCNamePrefix)
			newSet.Namespace = set.Namespace
			newSet.Labels = set.Labels
			newSet.Annotations = make(map[string]string)
			for k, v := range set.Annotations {
				newSet.Annotations[k] = v
			}
			newSet.Annotations[statefulSetTempAnnotation] = migapi.True
			newSet.Spec = *set.Spec.DeepCopy()
			zero := int32(0)
			newSet.Spec.Replicas = &zero
			if set.Annotations == nil {
				continue
			}
			if replicas, exist := set.Annotations[migapi.ReplicasAnnotation]; exist {
				number, err := strconv.Atoi(replicas)
				if err != nil {
					t.Log.Error(err, "failed finding replica count for statefulset",
						"namespace", set.Namespace, "statefulset", set.Name)
					failedStatefulSets = append(failedStatefulSets,
						fmt.Sprintf("%s/%s", set.Namespace, set.Name))
				} else {
					delete(set.Annotations, migapi.ReplicasAnnotation)
					restoredReplicas := int32(number)
					// Only change replica count if currently == 0
					if *set.Spec.Replicas == 0 {
						set.Spec.Replicas = &restoredReplicas
					}
				}
			}
			for i := range set.Spec.VolumeClaimTemplates {
				volumeTemplate := &set.Spec.VolumeClaimTemplates[i]
				formattedName := volumeTemplate.Name
				if t.rollback() {
					if _, exists := set.Annotations[statefulSetUpdateAnnotation]; exists {
						delete(set.Annotations, statefulSetUpdateAnnotation)
						matcher := regexp.MustCompile(fmt.Sprintf("-%s$", migapi.StorageConversionPVCNamePrefix))
						if matcher.MatchString(volumeTemplate.Name) {
							formattedName = matcher.ReplaceAllString(volumeTemplate.Name, "")
						}
					}
					if _, exists := set.Annotations[statefulSetTempAnnotation]; exists {
						t.Log.Error(fmt.Errorf("failed rolling back statefulset"),
							"namespace", set.Namespace, "statefulset", set.Name)
						failedStatefulSets = append(failedStatefulSets,
							fmt.Sprintf("%s/%s", set.Namespace, set.Name))
						continue
					}
				} else {
					if set.Annotations == nil {
						set.Annotations = make(map[string]string)
					}
					set.Annotations[statefulSetUpdateAnnotation] = migapi.True
					formattedName = fmt.Sprintf("%s-%s", volumeTemplate.Name, migapi.StorageConversionPVCNamePrefix)
				}
				updateContainerVolumeMounts := func(containers []v1.Container) {
					for i := range containers {
						container := &containers[i]
						for i := range container.VolumeMounts {
							volumeMount := &container.VolumeMounts[i]
							if volumeMount.Name == volumeTemplate.Name {
								volumeMount.Name = formattedName
							}
						}
					}
				}
				updateContainerVolumeMounts(set.Spec.Template.Spec.Containers)
				updateContainerVolumeMounts(set.Spec.Template.Spec.InitContainers)
				volumeTemplate.Name = formattedName
			}
			for _, volume := range set.Spec.Template.Spec.Volumes {
				updatePVCRef(volume.PersistentVolumeClaim, set.Namespace, mapping)
			}
			err = client.Create(context.TODO(), newSet)
			if err != nil {
				t.Log.Error(err, "failed creating temporary statefulset",
					"namespace", newSet.Namespace, "statefulset", newSet.Name)
				failedStatefulSets = append(failedStatefulSets,
					fmt.Sprintf("%s/%s", set.Namespace, set.Name))
				continue
			}
			err = client.Delete(context.TODO(), set)
			if err != nil {
				t.Log.Error(err, "failed deleting statefulset",
					"namespace", set.Namespace, "statefulset", set.Name)
				failedStatefulSets = append(failedStatefulSets,
					fmt.Sprintf("%s/%s", set.Namespace, set.Name))
				continue
			}
			err = client.Create(context.TODO(), set)
			if err != nil {
				t.Log.Error(err, "failed creating updated statefulset",
					"namespace", set.Namespace, "statefulset", set.Name)
				failedStatefulSets = append(failedStatefulSets,
					fmt.Sprintf("%s/%s", set.Namespace, set.Name))
				continue
			}
			err = client.Delete(context.TODO(), newSet)
			if err != nil {
				t.Log.Error(err, "failed deleting temporary statefulset",
					"namespace", set.Namespace, "statefulset", set.Name)
				failedStatefulSets = append(failedStatefulSets,
					fmt.Sprintf("%s/%s", set.Namespace, set.Name))
			}
		}
	}
	return
}

// swapDeploymentPVCRefs
func (t *Task) swapDeploymentPVCRefs(client k8sclient.Client, mapping pvcNameMapping) (failedDeployments []string) {
	for _, ns := range t.destinationNamespaces() {
		list := appsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			t.Log.Error(err, "failed listing deployments", "namespace", ns)
			for _, deployment := range list.Items {
				failedDeployments = append(failedDeployments,
					fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name))
			}
			continue
		}
		for _, deployment := range list.Items {
			isFailed := false
			// un-quiesce application
			if replicas, exist := deployment.Annotations[migapi.ReplicasAnnotation]; exist {
				number, err := strconv.Atoi(replicas)
				if err != nil {
					t.Log.Error(err, "failed finding replica count for deployment",
						"namespace", deployment.Namespace, "deployment", deployment.Name)
					isFailed = true
				} else {
					rolledOutReplicas := int32(number)
					deployment.Spec.Replicas = &rolledOutReplicas
				}
				if val, exists := deployment.Annotations[migapi.PausedAnnotation]; exists {
					if boolVal, err := strconv.ParseBool(val); err == nil {
						deployment.Spec.Paused = boolVal
					}
				}
			}
			// swap PVC refs
			for _, volume := range deployment.Spec.Template.Spec.Volumes {
				isFailed = updatePVCRef(volume.PersistentVolumeClaim, deployment.Namespace, mapping)
			}
			err = client.Update(context.TODO(), &deployment)
			if err != nil {
				t.Log.Error(err, "failed updating deployment",
					"namespace", deployment.Namespace, "deployment", deployment.Name)
				isFailed = true
			}
			if isFailed {
				failedDeployments = append(failedDeployments,
					fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name))
			}
		}
	}
	return
}

// swapDeploymentConfigPVCRefs
func (t *Task) swapDeploymentConfigPVCRefs(client k8sclient.Client, mapping pvcNameMapping) (failedDeploymentConfigs []string) {
	for _, ns := range t.destinationNamespaces() {
		list := ocappsv1.DeploymentConfigList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			t.Log.Error(err, "failed listing deploymentconfigs", "namespace", ns)
			for _, deploymentConfig := range list.Items {
				failedDeploymentConfigs = append(failedDeploymentConfigs,
					fmt.Sprintf("%s/%s", deploymentConfig.Namespace, deploymentConfig.Name))
			}
			continue
		}
		for _, deploymentConfig := range list.Items {
			isFailed := false
			// un-quiesce application
			if replicas, exist := deploymentConfig.Annotations[migapi.ReplicasAnnotation]; exist {
				number, err := strconv.Atoi(replicas)
				if err != nil {
					t.Log.Error(err, "failed finding replica count for deploymentconfig",
						"namespace", deploymentConfig.Namespace, "deploymentConfig", deploymentConfig.Name)
					isFailed = true
				} else {
					deploymentConfig.Spec.Replicas = int32(number)
				}
				if val, exists := deploymentConfig.Annotations[migapi.PausedAnnotation]; exists {
					if boolVal, err := strconv.ParseBool(val); err == nil {
						deploymentConfig.Spec.Paused = boolVal
					}
				}
			}
			// update PVC ref
			for _, volume := range deploymentConfig.Spec.Template.Spec.Volumes {
				isFailed = updatePVCRef(volume.PersistentVolumeClaim, deploymentConfig.Namespace, mapping)
			}
			err = client.Update(context.TODO(), &deploymentConfig)
			if err != nil {
				t.Log.Error(err,
					"failed updating deploymentconfig", "namespace", deploymentConfig.Namespace,
					"deploymentConfig", deploymentConfig.Name)
				isFailed = true
			}
			if isFailed {
				failedDeploymentConfigs = append(failedDeploymentConfigs,
					fmt.Sprintf("%s/%s", deploymentConfig.Namespace, deploymentConfig.Name))
			}
		}
	}
	return
}

// swapReplicaSetsPVCRefs
func (t *Task) swapReplicaSetsPVCRefs(client k8sclient.Client, mapping pvcNameMapping) (failedReplicasets []string) {
	for _, ns := range t.destinationNamespaces() {
		list := appsv1.ReplicaSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			t.Log.Error(err, "failed listing replicasets", "namespace", ns)
			for _, replicaset := range list.Items {
				failedReplicasets = append(failedReplicasets,
					fmt.Sprintf("%s/%s", replicaset.Namespace, replicaset.Name))
			}
			return
		}
		for _, replicaset := range list.Items {
			if len(replicaset.OwnerReferences) > 0 {
				t.Log.Info("swap pvc skipping ReplicaSet, has OwnerReferences",
					"namespace", replicaset.Namespace, "replicaset", replicaset.Name)
				continue
			}
			isFailed := false
			// un-quiesce application
			if replicas, exist := replicaset.Annotations[migapi.ReplicasAnnotation]; exist {
				number, err := strconv.Atoi(replicas)
				if err != nil {
					t.Log.Error(err, "failed finding replica count for replicaset",
						"namespace", replicaset.Namespace, "replicaset", replicaset.Name)
					isFailed = true
				} else {
					replicaCount := int32(number)
					replicaset.Spec.Replicas = &replicaCount
				}
			}
			// update PVC ref
			for _, volume := range replicaset.Spec.Template.Spec.Volumes {
				isFailed = updatePVCRef(volume.PersistentVolumeClaim, replicaset.Namespace, mapping)
			}
			err = client.Update(context.TODO(), &replicaset)
			if err != nil {
				t.Log.Error(err, "failed updating replicaset",
					"namespace", replicaset.Namespace, "replicaset", replicaset.Name)
				isFailed = true
			}
			if isFailed {
				failedReplicasets = append(failedReplicasets,
					fmt.Sprintf("%s/%s", replicaset.Namespace, replicaset.Name))
			}
		}
	}
	return
}

// swapDaemonSetsPVCRefs
func (t *Task) swapDaemonSetsPVCRefs(client k8sclient.Client, mapping pvcNameMapping) (failedDaemonSets []string) {
	for _, ns := range t.destinationNamespaces() {
		list := appsv1.DaemonSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			t.Log.Error(err, "failed listing daemonsets", "namespace", ns)
			for _, daemonSet := range list.Items {
				failedDaemonSets = append(failedDaemonSets,
					fmt.Sprintf("%s/%s", daemonSet.Namespace, daemonSet.Name))
			}
			continue
		}
		for _, daemonSet := range list.Items {
			isFailed := false
			if daemonSet.Annotations != nil {
				if selector, exist := daemonSet.Annotations[migapi.NodeSelectorAnnotation]; exist {
					nodeSelector := map[string]string{}
					err := json.Unmarshal([]byte(selector), &nodeSelector)
					if err != nil {
						t.Log.Error(err,
							"failed unmarshling nodeselector", "daemonset", daemonSet.Name, "namespace", ns, "nodeSelector", nodeSelector)
					} else {
						// Only change node selector if set to our quiesce nodeselector
						if _, isQuiesced := daemonSet.Spec.Template.Spec.NodeSelector[migapi.QuiesceNodeSelector]; isQuiesced {
							delete(daemonSet.Annotations, migapi.NodeSelectorAnnotation)
							daemonSet.Spec.Template.Spec.NodeSelector = nodeSelector
						}
					}
				}
			}
			for _, volume := range daemonSet.Spec.Template.Spec.Volumes {
				isFailed = updatePVCRef(volume.PersistentVolumeClaim, daemonSet.Namespace, mapping)
			}
			err = client.Update(context.TODO(), &daemonSet)
			if err != nil {
				t.Log.Error(err, "failed updating daemonset",
					"namespace", daemonSet.Namespace, "daemonset", daemonSet.Name)
				isFailed = true
			}
			if isFailed {
				failedDaemonSets = append(failedDaemonSets,
					fmt.Sprintf("%s/%s", daemonSet.Namespace, daemonSet.Name))
			}
		}
	}
	return
}

func isJobComplete(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete &&
			condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// swapJobsPVCRefs
func (t *Task) swapJobsPVCRefs(client k8sclient.Client, mapping pvcNameMapping) (failedJobs []string) {
	for _, ns := range t.destinationNamespaces() {
		list := batchv1.JobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			t.Log.Error(err, "failed listing jobs", "namespace", ns)
			for _, job := range list.Items {
				failedJobs = append(failedJobs,
					fmt.Sprintf("%s/%s", job.Namespace, job.Name))
			}
			continue
		}
		for i := range list.Items {
			oldJob := &list.Items[i]
			// if job is already complete, skip
			if isJobComplete(oldJob) {
				continue
			}
			isFailed := false
			job := &batchv1.Job{}
			job.Namespace = oldJob.Namespace
			job.ObjectMeta.Labels = oldJob.Labels
			job.ObjectMeta.Annotations = oldJob.Annotations
			delete(job.Labels, "job-name")
			delete(job.Labels, "controller-uid")
			jobSpec := oldJob.Spec.DeepCopy()
			if jobSpec != nil {
				job.Spec = *jobSpec
				job.Spec.Selector = nil
				delete(job.Spec.Template.Labels, "job-name")
				delete(job.Spec.Template.Labels, "controller-uid")
			} else {
				continue
			}
			job.Name = ""
			if len(oldJob.Name) > 60 {
				job.GenerateName = fmt.Sprintf("%s-", oldJob.Name[:60])
			} else {
				job.GenerateName = fmt.Sprintf("%s-", oldJob.Name)
			}
			if job.Annotations != nil {
				if replicas, exist := job.Annotations[migapi.ReplicasAnnotation]; exist {
					number, err := strconv.Atoi(replicas)
					if err != nil {
						t.Log.Error(err,
							"failed finding replicas of job", "namespace", ns, "job", job.Name)
						isFailed = true
					} else {
						delete(job.Annotations, migapi.ReplicasAnnotation)
						parallelReplicas := int32(number)
						// Only change parallelism if currently == 0
						if *job.Spec.Parallelism == 0 {
							job.Spec.Parallelism = &parallelReplicas
						}
					}
				}
			}
			for _, volume := range job.Spec.Template.Spec.Volumes {
				isFailed = updatePVCRef(volume.PersistentVolumeClaim, job.Namespace, mapping)
			}
			err := client.Create(context.TODO(), job)
			if err != nil {
				t.Log.Error(err, "failed updating job",
					"namespace", oldJob.Namespace, "job", oldJob.Name)
				isFailed = true
			}
			if isFailed {
				failedJobs = append(failedJobs,
					fmt.Sprintf("%s/%s", oldJob.Namespace, oldJob.Name))
			}
		}
	}
	return
}

// swapCronJobsPVCRefs
func (t *Task) swapCronJobsPVCRefs(client k8sclient.Client, mapping pvcNameMapping) (failedCronJobs []string) {
	for _, ns := range t.destinationNamespaces() {
		list := batchv1beta.CronJobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			&list,
			options)
		if err != nil {
			t.Log.Error(err, "failed listing cronjobs", "namespace", ns)
			for _, job := range list.Items {
				failedCronJobs = append(failedCronJobs,
					fmt.Sprintf("%s/%s", job.Namespace, job.Name))
			}
			continue
		}
		for _, cronJob := range list.Items {
			isFailed := false
			// unquiesce cronjob
			if cronJob.Annotations != nil {
				// Only unsuspend if our suspend annotation is present
				if _, exist := cronJob.Annotations[migapi.SuspendAnnotation]; exist {
					delete(cronJob.Annotations, migapi.SuspendAnnotation)
					cronJob.Spec.Suspend = ptr.To[bool](false)
				}
			}
			for _, volume := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes {
				isFailed = updatePVCRef(volume.PersistentVolumeClaim, cronJob.Namespace, mapping)
			}
			err := client.Update(context.TODO(), &cronJob)
			if err != nil {
				t.Log.Error(err, "failed updating cronjob",
					"namespace", cronJob.Namespace, "cronjob", cronJob.Name)
				isFailed = true
			}
			if isFailed {
				failedCronJobs = append(failedCronJobs,
					fmt.Sprintf("%s/%s", cronJob.Namespace, cronJob.Name))
			}
		}
	}
	return
}

func (t *Task) swapVirtualMachinePVCRefs(client k8sclient.Client, restClient rest.Interface, mapping pvcNameMapping) (failedVirtualMachines []string) {
	for _, ns := range t.destinationNamespaces() {
		list := &virtv1.VirtualMachineList{}
		if err := client.List(context.TODO(), list, k8sclient.InNamespace(ns)); err != nil {
			if k8smeta.IsNoMatchError(err) {
				continue
			}
			t.Log.Error(err, "failed listing virtual machines", "namespace", ns)
			continue
		}
		for _, vm := range list.Items {
			retryCount := 1
			retry := true
			for retry && retryCount <= 3 {
				message, err := t.swapVirtualMachinePVCRef(client, restClient, &vm, mapping)
				if err != nil && !k8serrors.IsConflict(err) {
					failedVirtualMachines = append(failedVirtualMachines, message)
					return
				} else if k8serrors.IsConflict(err) {
					t.Log.Info("Conflict updating VM, retrying after reloading VM resource")
					// Conflict, reload VM and try again
					if err := client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: ns, Name: vm.Name}, &vm); err != nil {
						failedVirtualMachines = append(failedVirtualMachines, fmt.Sprintf("failed reloading %s/%s", ns, vm.Name))
					}
					retryCount++
				} else {
					retry = false
					if message != "" {
						failedVirtualMachines = append(failedVirtualMachines, message)
					}
				}
			}
		}
	}
	return
}

func (t *Task) swapVirtualMachinePVCRef(client k8sclient.Client, restClient rest.Interface, vm *virtv1.VirtualMachine, mapping pvcNameMapping) (string, error) {
	if vm.Spec.Template == nil {
		return "", nil
	}
	for i, volume := range vm.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			if isFailed := updatePVCRef(&vm.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.PersistentVolumeClaimVolumeSource, vm.Namespace, mapping); isFailed {
				return fmt.Sprintf("%s/%s", vm.Namespace, vm.Name), nil
			}
		}
		if volume.DataVolume != nil {
			isFailed, err := updateDataVolumeRef(client, vm.Spec.Template.Spec.Volumes[i].DataVolume, vm.Namespace, mapping, t.Log)
			if err != nil || isFailed {
				return fmt.Sprintf("%s/%s", vm.Namespace, vm.Name), err
			}
			// Update datavolume template if it exists.
			for i, dvt := range vm.Spec.DataVolumeTemplates {
				if destinationDVName, exists := mapping.Get(vm.Namespace, dvt.Name); exists {
					vm.Spec.DataVolumeTemplates[i].Name = destinationDVName
				}
			}
		}
	}
	if !isVMActive(vm, client) {
		if err := client.Update(context.Background(), vm); err != nil {
			t.Log.Error(err, "failed updating VM", "namespace", vm.Namespace, "name", vm.Name)
			return fmt.Sprintf("%s/%s", vm.Namespace, vm.Name), err
		}
		if err := client.Get(context.Background(), k8sclient.ObjectKey{Namespace: vm.Namespace, Name: vm.Name}, vm); err != nil {
			t.Log.Error(err, "failed getting VM", "namespace", vm.Namespace, "name", vm.Name)
			return fmt.Sprintf("%s/%s", vm.Namespace, vm.Name), err
		}
		if shouldStartVM(vm) {
			if err := t.startVM(vm, client, restClient); err != nil {
				t.Log.Error(err, "failed starting VM", "namespace", vm.Namespace, "name", vm.Name)
				return fmt.Sprintf("%s/%s", vm.Namespace, vm.Name), err
			}
		}
	}
	return "", nil
}

// updatePVCRef given a PVCSource, namespace and a mapping of pvc names, swaps the claim
// present in the pvc source with the mapped pvc name found in the mapping
// returns whether the swap was successful or not, true is failure, false is success
func updatePVCRef(pvcSource *v1.PersistentVolumeClaimVolumeSource, ns string, mapping pvcNameMapping) bool {
	if pvcSource != nil {
		originalName := pvcSource.ClaimName
		if destinationPVCName, exists := mapping.Get(ns, originalName); exists {
			pvcSource.ClaimName = destinationPVCName
		} else {
			// attempt to figure out whether the current PVC reference
			// already points to the new migrated PVC. This is needed to
			// guarantee idempotency of the operation
			if !mapping.ExistsAsValue(originalName) {
				return true
			}
		}
	}
	return false
}

func updateDataVolumeRef(client k8sclient.Client, dv *virtv1.DataVolumeSource, ns string, mapping pvcNameMapping, log logr.Logger) (bool, error) {
	if dv != nil {
		log.Info("Updating DataVolume reference", "namespace", ns, "name", dv.Name)
		originalName := dv.Name
		originalDv := &cdiv1.DataVolume{}
		if err := client.Get(context.Background(), k8sclient.ObjectKey{Namespace: ns, Name: originalName}, originalDv); err != nil {
			log.Error(err, "failed getting DataVolume", "namespace", ns, "name", originalName)
			return true, err
		}

		if destinationDVName, exists := mapping.Get(ns, originalName); exists {
			dv.Name = destinationDVName
			err := dvmc.CreateNewDataVolume(client, originalDv.Name, destinationDVName, ns, log)
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "failed creating DataVolume", "namespace", ns, "name", destinationDVName)
				return true, err
			}
		} else {
			// attempt to figure out whether the current DV reference
			// already points to the new migrated PVC. This is needed to
			// guarantee idempotency of the operation
			if !mapping.ExistsAsValue(originalName) {
				return true, nil
			}
		}
	}
	return false, nil
}
