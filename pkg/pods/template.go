package pods

import (
	"context"

	"github.com/konveyor/mig-controller/pkg/compat"
	ocappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	virtv1 "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ListTemplatePods - get list of pod templates, associated with a plan resource
func ListTemplatePods(client compat.Client, namespaces []string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}

	for _, ns := range namespaces {
		newPods, err := listDeploymentTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listDeploymentConfigTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listReplicationControllerTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listDaemonSetTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listStatefulSetTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listReplicaSetTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listCronJobTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listJobTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

		newPods, err = listVirtualMachineTemplatePodsForNamespace(client, ns)
		if err != nil {
			return nil, err
		}
		pods = append(pods, newPods...)

	}
	return pods, nil
}

func listDeploymentTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := appsv1.DeploymentList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, deployment := range list.Items {
		podTemplate := deployment.Spec.Template
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployment.GetName(),
				Namespace: deployment.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       deployment.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listDeploymentConfigTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := ocappsv1.DeploymentConfigList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, deploymentConfig := range list.Items {
		podTemplate := deploymentConfig.Spec.Template
		if podTemplate == nil {
			continue
		}
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentConfig.GetName(),
				Namespace: deploymentConfig.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "DeploymentConfig",
						Name:       deploymentConfig.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listReplicationControllerTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := corev1.ReplicationControllerList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, replicationController := range list.Items {
		podTemplate := replicationController.Spec.Template
		if podTemplate == nil {
			continue
		}
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      replicationController.GetName(),
				Namespace: replicationController.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "DeploymentConfig",
						Name:       replicationController.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listDaemonSetTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := appsv1.DaemonSetList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, daemonSet := range list.Items {
		podTemplate := daemonSet.Spec.Template
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonSet.GetName(),
				Namespace: daemonSet.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "DaemonSet",
						Name:       daemonSet.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listStatefulSetTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := appsv1.StatefulSetList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, statefulSet := range list.Items {
		podTemplate := statefulSet.Spec.Template
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      statefulSet.GetName(),
				Namespace: statefulSet.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       statefulSet.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listReplicaSetTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := appsv1.ReplicaSetList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, replicaSet := range list.Items {
		podTemplate := replicaSet.Spec.Template
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      replicaSet.GetName(),
				Namespace: replicaSet.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       replicaSet.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listJobTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := batchv1.JobList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, job := range list.Items {
		podTemplate := job.Spec.Template
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      job.GetName(),
				Namespace: job.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "batch/v1",
						Kind:       "Job",
						Name:       job.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listCronJobTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := batchv1beta.CronJobList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	for _, cronJob := range list.Items {
		podTemplate := cronJob.Spec.JobTemplate.Spec.Template
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cronJob.GetName(),
				Namespace: cronJob.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "batch/v1",
						Kind:       "CronJob",
						Name:       cronJob.GetName(),
					},
				},
			},
			Spec: podTemplate.Spec,
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func listVirtualMachineTemplatePodsForNamespace(client k8sclient.Client, ns string) ([]corev1.Pod, error) {
	pods := []corev1.Pod{}
	list := virtv1.VirtualMachineList{}
	err := client.List(context.TODO(), &list, k8sclient.InNamespace(ns))
	if err != nil {
		if meta.IsNoMatchError(err) {
			return pods, nil
		}
		return nil, err
	}
	for _, vm := range list.Items {
		vmi := vm.Spec.Template
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vm.GetName(),
				Namespace: vm.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "kubevirt.io/v1",
						Kind:       "VirtualMachineInstance",
					},
				},
			},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{},
			},
		}
		for _, vmVolume := range vmi.Spec.Volumes {
			if vmVolume.PersistentVolumeClaim == nil && vmVolume.DataVolume == nil {
				continue
			}
			claimName := ""
			if vmVolume.PersistentVolumeClaim != nil {
				claimName = vmVolume.PersistentVolumeClaim.ClaimName
			} else if vmVolume.DataVolume != nil {
				claimName = vmVolume.DataVolume.Name
			} else {
				continue
			}
			volume := corev1.Volume{
				Name: vmVolume.Name,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: claimName,
					},
				},
			}
			pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
		}
		pods = append(pods, pod)
	}
	return pods, nil
}
