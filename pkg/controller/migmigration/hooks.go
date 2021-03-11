package migmigration

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const HookJobFailedLimit = 6
const BackoffLimitExceededError = "BackoffLimitExceeded"

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
		t.Log.Info("Found MigHook ref [%v/%v] attached for phase=[%v], starting hook job.",
			hook.Reference.Namespace, hook.Reference.Name, hookPhase)
		err = t.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      hook.Reference.Name,
				Namespace: hook.Reference.Namespace,
			},
			&migHook)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		t.Log.Info("Getting k8s client for MigHook [%v/%v]",
			migHook.Namespace, migHook.Name)
		client, err = t.getHookClient(migHook)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		svc := corev1.ServiceAccount{}
		ref := types.NamespacedName{
			Namespace: hook.ExecutionNamespace,
			Name:      hook.ServiceAccount,
		}
		t.Log.Info("Getting executor ServiceAccount [%v/%v] for MigHook [%v/%v]",
			svc.Namespace, svc.Name, migHook.Namespace, migHook.Name)
		err = client.Get(context.TODO(), ref, &svc)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		t.Log.Info("Building Job resource definition for MigHook [%v/%v]",
			migHook.Namespace, migHook.Name)
		job, err := t.prepareJob(hook, migHook, client)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		t.Log.Info("Creating Job [%v/%v] for MigHook [%v/%v]",
			job.Namespace, job.Name, migHook.Namespace, migHook.Name)
		result, err := t.ensureJob(job, hook, migHook, client)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		return result, nil
	}
	t.Log.Info(fmt.Sprintf("No hook attached to MigPlan for HookPhase=[%v], continuing.",
		hookPhase))
	return true, nil
}

func (t *Task) stopHookJobs() (bool, error) {
	var client k8sclient.Client
	var err error

	migHook := migapi.MigHook{}
	for _, hook := range t.PlanResources.MigPlan.Spec.Hooks {
		t.Log.Info("Found MigHook ref [%v/%v], stopping hook job(s).",
			hook.Reference.Namespace, hook.Reference.Name)
		if hook.Reference == nil {
			continue
		}

		t.Log.Info("Getting MigHook [%v/%v]",
			hook.Reference.Namespace, hook.Reference.Name)
		err = t.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      hook.Reference.Name,
				Namespace: hook.Reference.Namespace,
			},
			&migHook)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		t.Log.Info("Getting k8s client for MigHook [%v/%v]",
			migHook.Namespace, migHook.Name)
		client, err = t.getHookClient(migHook)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		// Get the job for the hook and kill it.
		t.Log.Info("Attempting to kill job for MigHook [%v/%v] phase=[%v]",
			hook.Reference.Namespace, hook.Reference.Name, hook.Phase)
		runningJob, err := migHook.GetPhaseJob(client, hook.Phase, string(t.Owner.UID))
		if runningJob == nil && err == nil {
			// No active Job for hook
			t.Log.Info("No active job found for MigHook [%v/%v] for phase=[%v]. Continuing.",
				hook.Reference.Namespace, hook.Reference.Name, hook.Phase)
			continue
		} else {
			// Job found
			t.Log.Info("Deleting hook job [%v/%v] found for MigHook [%v/%v] phase=[%v]. Continuing.",
				runningJob.Namespace, runningJob.Name,
				hook.Reference.Namespace, hook.Reference.Name, hook.Phase)
			err = client.Delete(context.TODO(), runningJob,
				k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
			if err != nil {
				t.Log.Error(err, fmt.Sprintf("Job %s could not be deleted", runningJob.Name))
			}
		}

	}
	return true, nil
}

func (t *Task) ensureJob(job *batchv1.Job, hook migapi.MigPlanHook, migHook migapi.MigHook, client k8sclient.Client) (bool, error) {
	runningJob, err := migHook.GetPhaseJob(client, hook.Phase, string(t.Owner.UID))
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
		t.setProgress([]string{
			fmt.Sprintf("Job %s/%s: Failed", runningJob.Namespace, runningJob.Name)})
		return false, err
	} else if len(runningJob.Status.Conditions) > 0 && runningJob.Status.Conditions[0].Reason == BackoffLimitExceededError {
		err := fmt.Errorf("Hook job %s failed.", runningJob.Name)
		t.setProgress([]string{
			fmt.Sprintf("Job %s/%s: Failed", runningJob.Namespace, runningJob.Name)})
		return false, err
	} else if runningJob.Status.Succeeded == 1 {
		t.setProgress([]string{
			fmt.Sprintf("Job %s/%s: Succeeded", runningJob.Namespace, runningJob.Name)})
		return true, nil
	} else {
		t.setProgress([]string{
			fmt.Sprintf("Job %s/%s: Running", runningJob.Namespace, runningJob.Name)})
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

		phaseConfigMap, err := migHook.GetPhaseConfigMap(client, hook.Phase, string(t.Owner.UID))
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
			return nil, liberr.Wrap(err)
		}
	default:
		err := fmt.Errorf("targetCluster must be 'source' or 'destination'. %s unknown", migHook.Spec.TargetCluster)
		return nil, liberr.Wrap(err)
	}
	return client, nil
}

func (t *Task) configMapTemplate(hook migapi.MigPlanHook, migHook migapi.MigHook) (*corev1.ConfigMap, error) {

	labels := migHook.GetCorrelationLabels()
	labels[migapi.HookPhaseLabel] = hook.Phase
	labels[migapi.HookOwnerLabel] = string(t.Owner.UID)

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
		Data: map[string]string{
			"playbook.yml": string(playbookData),
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
	labels[migapi.HookPhaseLabel] = hook.Phase
	labels[migapi.HookOwnerLabel] = string(t.Owner.UID)

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
							Env: []corev1.EnvVar{
								{
									Name:  "MIGRATION_NAMESPACES",
									Value: strings.Join(t.PlanResources.MigPlan.Spec.Namespaces, ","),
								},
								{
									Name:  "MIGRATION_PLAN_NAME",
									Value: t.PlanResources.MigPlan.Name,
								},
							},
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
