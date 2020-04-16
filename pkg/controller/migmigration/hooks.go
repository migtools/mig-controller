package migmigration

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const HookJobFailedLimit = 5

func (t *Task) runHooks(hookPhase string) (bool, error) {
	hook := migapi.MigPlanHook{}
	var client k8sclient.Client
	var err error

	for _, h := range t.PlanResources.MigPlan.Spec.Hooks {
		if h.Phase == hookPhase {
			hook = h
		}
	}

	migHook := migapi.MigHook{}

	if hook.Reference != nil {
		err = t.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      hook.Reference.Name,
				Namespace: hook.Reference.Namespace,
			},
			&migHook)
		if err != nil {
			log.Trace(err)
			return false, err
		}

		client, err = t.getHookClient(migHook)
		if err != nil {
			log.Trace(err)
			return false, err
		}

		job, err := t.prepareJob(hook, migHook, client)
		if err != nil {
			log.Trace(err)
			return false, err
		}

		result, err := t.ensureJob(job, hook, migHook, client)
		if err != nil {
			log.Trace(err)
			return false, err
		}

		return result, nil
	}
	return true, nil
}

func (t *Task) ensureJob(job *batchv1.Job, hook migapi.MigPlanHook, migHook migapi.MigHook, client k8sclient.Client) (bool, error) {
	runningJob, err := migHook.GetPhaseJob(client, hook.Phase)
	if runningJob == nil && err == nil {
		err = client.Create(context.TODO(), job)
		if err != nil {
			return false, err
		}
		return false, nil
	} else if err != nil {
		return false, err
	} else if runningJob.Status.Failed >= HookJobFailedLimit {
		err := fmt.Errorf("Hook job %s failed.", runningJob.Name)
		return false, err
	} else if runningJob.Status.Succeeded == 1 {
		return true, nil
	} else {
		return false, nil
	}
}

func (t *Task) prepareJob(hook migapi.MigPlanHook, migHook migapi.MigHook, client k8sclient.Client) (*batchv1.Job, error) {
	job := &batchv1.Job{}

	if migHook.Spec.Custom {
		job = t.baseJobTemplate(hook, migHook)
	} else {

		configMap, err := t.configMapTemplate(hook, migHook)
		if err != nil {
			return nil, err
		}

		phaseConfigMap, err := migHook.GetPhaseConfigMap(client, hook.Phase)
		if phaseConfigMap == nil && err == nil {

			err = client.Create(context.TODO(), configMap)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		job = t.playbookJobTemplate(hook, migHook, configMap.Name)
	}

	return job, nil
}

func (t *Task) getHookClient(migHook migapi.MigHook) (k8sclient.Client, error) {
	var client k8sclient.Client
	var err error

	switch migHook.Spec.TargetCluster {
	case "destination":
		client, err = t.getDestinationClient()
		if err != nil {
			return nil, err
		}
	case "source":
		client, err = t.getSourceClient()
		if err != nil {
			log.Trace(err)
			return nil, err
		}
	default:
		err := fmt.Errorf("targetCluster must be 'source' or 'destination'. %s unknown", migHook.Spec.TargetCluster)
		log.Trace(err)
		return nil, err
	}
	return client, nil
}

func (t *Task) configMapTemplate(hook migapi.MigPlanHook, migHook migapi.MigHook) (*corev1.ConfigMap, error) {

	labels := migHook.GetCorrelationLabels()
	labels["phase"] = hook.Phase

	playbookData, err := base64.StdEncoding.DecodeString(migHook.Spec.Playbook)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    hook.ExecutionNamespace,
			GenerateName: strings.ToLower(t.PlanResources.MigPlan.Name + "-" + hook.Phase + "-"),
			Labels:       labels,
		},
		BinaryData: map[string][]byte{
			"playbook.yml": []byte(playbookData),
		},
	}, nil
}

func (t *Task) playbookJobTemplate(hook migapi.MigPlanHook, migHook migapi.MigHook, configMap string) *batchv1.Job {
	jobTemplate := t.baseJobTemplate(hook, migHook)

	jobTemplate.Spec.Template.Spec.Containers[0].Command = []string{
		"/bin/entrypoint",
		"ansible-runner",
		"-p",
		"/tmp/playbook/playbook.yml",
		"run",
		"/tmp/runner",
	}

	jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "playbook",
			MountPath: "/tmp/playbook",
		},
	}

	jobTemplate.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "playbook",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMap,
					},
				},
			},
		},
	}

	return jobTemplate
}

func (t *Task) baseJobTemplate(hook migapi.MigPlanHook, migHook migapi.MigHook) *batchv1.Job {
	deadlineSeconds := int64(1800)

	if migHook.Spec.ActiveDeadlineSeconds != 0 {
		deadlineSeconds = migHook.Spec.ActiveDeadlineSeconds
	}

	labels := migHook.GetCorrelationLabels()
	labels["phase"] = hook.Phase

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    hook.ExecutionNamespace,
			GenerateName: strings.ToLower(t.PlanResources.MigPlan.Name + "-" + hook.Phase + "-"),
			Labels:       labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  strings.ToLower(t.PlanResources.MigPlan.Name + "-" + hook.Phase),
							Image: migHook.Spec.Image,
						},
					},
					RestartPolicy:         "OnFailure",
					ServiceAccountName:    hook.ServiceAccount,
					ActiveDeadlineSeconds: &deadlineSeconds,
				},
			},
		},
	}
}
