package velerorunner

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	appsv1 "github.com/openshift/api/apps/v1"
	imagev1 "github.com/openshift/api/image/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the migration registry on the source cluster has been created
// and has the proper settings.
func (t *Task) ensureSrcMigRegistry() error {
	t.SrcRegistryResources = &MigRegistryResources{}
	client, err := t.getSourceClient()
	if err != nil {
		return err
	}
	return t.ensureMigRegistry(client, t.SrcRegistryResources)
}

// Ensure the migration registry on the destination cluster has been created
// and has the proper settings.
func (t *Task) ensureDestMigRegistry() error {
	t.DestRegistryResources = &MigRegistryResources{}
	client, err := t.getDestinationClient()
	if err != nil {
		return err
	}
	return t.ensureMigRegistry(client, t.DestRegistryResources)
}

// Ensure the migration registry on the specified cluster has been created
// and has the proper settings.
func (t *Task) ensureMigRegistry(client k8sclient.Client, registryResources *MigRegistryResources) error {
	err := t.ensureRegistrySecret(client, registryResources)
	if err != nil {
		return err
	}

	err = t.ensureRegistryImageStream(client, registryResources)
	if err != nil {
		return err
	}

	err = t.ensureRegistryDC(client, registryResources)
	if err != nil {
		return err
	}
	err = t.ensureRegistryService(client, registryResources)
	if err != nil {
		return err
	}
	return nil
}

// Ensure the credentials secret for the migration registry on the specified cluster has been created
func (t *Task) ensureRegistrySecret(client k8sclient.Client, registryResources *MigRegistryResources) error {
	newSecret, err := t.buildRegistrySecret()
	if err != nil {
		return err
	}
	foundSecret, err := t.getRegistrySecret(client)
	if err != nil {
		return err
	}
	if foundSecret == nil {
		registryResources.CredSecret = newSecret
		err = client.Create(context.TODO(), newSecret)
		if err != nil {
			return err
		}
		return nil
	}
	registryResources.CredSecret = foundSecret
	if !t.equalsRegistrySecret(newSecret, foundSecret) {
		t.updateRegistrySecret(foundSecret)
		err = client.Update(context.TODO(), foundSecret)
		if err != nil {
			return err
		}
	}
	return nil
}

// Build a credentials Secret as desired for the source cluster.
func (t *Task) buildRegistrySecret() (*kapi.Secret, error) {
	secret := &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.PlanResources.MigPlan.GetCorrelationLabels(),
			GenerateName: t.PlanResources.MigPlan.GetName() + "-registry-",
			Namespace:    VeleroNamespace,
		},
	}
	err := t.updateRegistrySecret(secret)
	return secret, err
}

// Update a Registry credentials secret as desired for the specified cluster.
func (t *Task) updateRegistrySecret(secret *kapi.Secret) error {
	credSecret, err := t.PlanResources.MigStorage.Spec.BackupStorageConfig.GetCredsSecret(t.Client)
	if err != nil {
		return err
	}
	if credSecret == nil {
		return migapi.CredSecretNotFound
	}
	secret.Data = map[string][]byte{
		"access_key": []byte(credSecret.Data[migapi.AwsAccessKeyId]),
		"secret_key": []byte(credSecret.Data[migapi.AwsSecretAccessKey]),
	}
	return nil
}

// Get an existing credentials Secret on the source cluster.
func (t *Task) getRegistrySecret(client k8sclient.Client) (*kapi.Secret, error) {
	list := kapi.SecretList{}
	labels := t.PlanResources.MigPlan.GetCorrelationLabels()
	err := client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Determine if two registry credentials secrets are equal.
// Returns `true` when equal.
func (t *Task) equalsRegistrySecret(a, b *kapi.Secret) bool {
	return reflect.DeepEqual(a.Data, b.Data)
}

// Ensure the imagestream for the migration registry on the specified cluster has been created
func (t *Task) ensureRegistryImageStream(client k8sclient.Client, registryResources *MigRegistryResources) error {
	registrySecret := registryResources.CredSecret
	if registrySecret == nil {
		return errors.New("migration registry credentials not found")
	}
	newImageStream, err := t.buildRegistryImageStream(registrySecret.GetName())
	if err != nil {
		return err
	}
	foundImageStream, err := t.getRegistryImageStream(client)
	if err != nil {
		return err
	}
	if foundImageStream == nil {
		registryResources.ImageStream = newImageStream
		err = client.Create(context.TODO(), newImageStream)
		if err != nil {
			return err
		}
		return nil
	}
	registryResources.ImageStream = foundImageStream
	if !t.equalsRegistryImageStream(newImageStream, foundImageStream) {
		t.updateRegistryImageStream(foundImageStream)
		err = client.Update(context.TODO(), foundImageStream)
		if err != nil {
			return err
		}
	}
	return nil
}

// Build a Registry ImageStream as desired for the source cluster.
func (t *Task) buildRegistryImageStream(name string) (*imagev1.ImageStream, error) {
	labels := t.PlanResources.MigPlan.GetCorrelationLabels()
	labels["app"] = name
	imagestream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      name,
			Namespace: VeleroNamespace,
		},
	}
	err := t.updateRegistryImageStream(imagestream)
	return imagestream, err
}

// Update a Registry ImageStream as desired for the specified cluster.
func (t *Task) updateRegistryImageStream(imagestream *imagev1.ImageStream) error {
	imagestream.Spec = imagev1.ImageStreamSpec{
		LookupPolicy: imagev1.ImageLookupPolicy{Local: false},
		Tags: []imagev1.TagReference{
			imagev1.TagReference{
				Name: "2",
				Annotations: map[string]string{
					"openshift.io/imported-from": "registry:2",
				},
				From: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "registry:2",
				},
				Generation:      nil,
				ImportPolicy:    imagev1.TagImportPolicy{},
				ReferencePolicy: imagev1.TagReferencePolicy{Type: ""},
			},
		},
	}
	return nil
}

// Get an existing registry ImageStream on the specifiedcluster.
func (t *Task) getRegistryImageStream(client k8sclient.Client) (*imagev1.ImageStream, error) {
	list := imagev1.ImageStreamList{}
	labels := t.PlanResources.MigPlan.GetCorrelationLabels()
	err := client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Determine if two imagestreams are equal.
// Returns `true` when equal.
func (t *Task) equalsRegistryImageStream(a, b *imagev1.ImageStream) bool {
	if len(a.Spec.Tags) != len(b.Spec.Tags) {
		return false
	}
	for i, tag := range a.Spec.Tags {
		if !(reflect.DeepEqual(tag.Name, b.Spec.Tags[i].Name) &&
			reflect.DeepEqual(tag.From, b.Spec.Tags[i].From)) {
			return false
		}
	}
	return true
}

// Ensure the deploymentconfig for the migration registry on the specified cluster has been created
func (t *Task) ensureRegistryDC(client k8sclient.Client, registryResources *MigRegistryResources) error {
	registrySecret := registryResources.CredSecret
	if registrySecret == nil {
		return errors.New("migration registry credentials not found")
	}
	name := registrySecret.GetName()
	// We need the src cluster's registry secret to grab the name to use for the REGISTRY_STORAGE root dir
	// since src and dest clusters must use the same dir name here, even if the resource names differ
	srcRegistrySecret := t.SrcRegistryResources.CredSecret
	if registrySecret == nil {
		return errors.New("src migration registry credentials not found")
	}
	srcName := srcRegistrySecret.GetName()
	newDC, err := t.buildRegistryDC(name, srcName)
	if err != nil {
		return err
	}
	foundDC, err := t.getRegistryDC(client)
	if err != nil {
		return err
	}
	if foundDC == nil {
		registryResources.DeploymentConfig = newDC
		err = client.Create(context.TODO(), newDC)
		if err != nil {
			return err
		}
		return nil
	}
	registryResources.DeploymentConfig = foundDC
	if !t.equalsRegistryDC(newDC, foundDC) {
		t.updateRegistryDC(foundDC, name, srcName)
		err = client.Update(context.TODO(), foundDC)
		if err != nil {
			return err
		}
	}
	return nil
}

// Build a Registry DeploymentConfig as desired for the source cluster.
func (t *Task) buildRegistryDC(name, srcName string) (*appsv1.DeploymentConfig, error) {
	labels := t.PlanResources.MigPlan.GetCorrelationLabels()
	labels["app"] = name
	deploymentconfig := &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      name,
			Namespace: VeleroNamespace,
		},
	}
	err := t.updateRegistryDC(deploymentconfig, name, srcName)
	return deploymentconfig, err
}

// Update a Registry DeploymentConfig as desired for the specified cluster.
func (t *Task) updateRegistryDC(deploymentconfig *appsv1.DeploymentConfig, name, srcName string) error {
	deploymentconfig.Spec = appsv1.DeploymentConfigSpec{
		Replicas: 1,
		Selector: map[string]string{
			"app":              name,
			"deploymentconfig": name,
		},
		Strategy: appsv1.DeploymentStrategy{Resources: kapi.ResourceRequirements{}},
		Template: &kapi.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{},
				Labels: map[string]string{
					"app":              name,
					"deploymentconfig": name,
				},
			},
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					kapi.Container{
						Env: []kapi.EnvVar{
							kapi.EnvVar{
								Name:  "REGISTRY_STORAGE",
								Value: "s3",
							},
							kapi.EnvVar{
								Name: "REGISTRY_STORAGE_S3_ACCESSKEY",
								ValueFrom: &kapi.EnvVarSource{
									SecretKeyRef: &kapi.SecretKeySelector{
										LocalObjectReference: kapi.LocalObjectReference{Name: name},
										Key:                  "access_key",
									},
								},
							},
							kapi.EnvVar{
								Name:  "REGISTRY_STORAGE_S3_BUCKET",
								Value: t.PlanResources.MigStorage.Spec.BackupStorageConfig.AwsBucketName,
							},
							kapi.EnvVar{
								Name:  "REGISTRY_STORAGE_S3_REGION",
								Value: t.PlanResources.MigStorage.Spec.BackupStorageConfig.AwsRegion,
							},
							kapi.EnvVar{
								Name:  "REGISTRY_STORAGE_S3_ROOTDIRECTORY",
								Value: "/" + srcName,
							},
							kapi.EnvVar{
								Name: "REGISTRY_STORAGE_S3_SECRETKEY",
								ValueFrom: &kapi.EnvVarSource{
									SecretKeyRef: &kapi.SecretKeySelector{
										LocalObjectReference: kapi.LocalObjectReference{Name: name},
										Key:                  "secret_key",
									},
								},
							},
						},
						Image: "registry:2",
						Name:  name,
						Ports: []kapi.ContainerPort{
							kapi.ContainerPort{
								ContainerPort: 5000,
								Protocol:      kapi.ProtocolTCP,
							},
						},
						Resources: kapi.ResourceRequirements{},
						VolumeMounts: []kapi.VolumeMount{
							kapi.VolumeMount{
								MountPath: "/var/lib/registry",
								Name:      name + "-volume-1",
							},
						},
					},
				},
				Volumes: []kapi.Volume{
					kapi.Volume{
						Name:         name + "-volume-1",
						VolumeSource: kapi.VolumeSource{EmptyDir: &kapi.EmptyDirVolumeSource{}},
					},
				},
			},
		},
		Test: false,
		Triggers: appsv1.DeploymentTriggerPolicies{
			appsv1.DeploymentTriggerPolicy{
				Type: appsv1.DeploymentTriggerOnConfigChange,
			},
		},
	}
	return nil
}

// Get an existing registry DeploymentConfig on the specifiedcluster.
func (t *Task) getRegistryDC(client k8sclient.Client) (*appsv1.DeploymentConfig, error) {
	list := appsv1.DeploymentConfigList{}
	labels := t.PlanResources.MigPlan.GetCorrelationLabels()
	err := client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Determine if two deploymentconfigs are equal.
// Returns `true` when equal.
func (t *Task) equalsRegistryDC(a, b *appsv1.DeploymentConfig) bool {
	if !(reflect.DeepEqual(a.Spec.Replicas, b.Spec.Replicas) &&
		reflect.DeepEqual(a.Spec.Selector, b.Spec.Selector) &&
		reflect.DeepEqual(a.Spec.Template.ObjectMeta, b.Spec.Template.ObjectMeta) &&
		reflect.DeepEqual(a.Spec.Template.Spec.Volumes, b.Spec.Template.Spec.Volumes) &&
		len(a.Spec.Template.Spec.Containers) == len(b.Spec.Template.Spec.Containers) &&
		len(a.Spec.Triggers) == len(b.Spec.Triggers)) {
		return false
	}
	for i, container := range a.Spec.Template.Spec.Containers {
		if !(reflect.DeepEqual(container.Env, b.Spec.Template.Spec.Containers[i].Env) &&
			reflect.DeepEqual(container.Name, b.Spec.Template.Spec.Containers[i].Name) &&
			reflect.DeepEqual(container.Ports, b.Spec.Template.Spec.Containers[i].Ports) &&
			reflect.DeepEqual(container.VolumeMounts, b.Spec.Template.Spec.Containers[i].VolumeMounts)) {
			return false
		}
	}
	for i, trigger := range a.Spec.Triggers {
		if !reflect.DeepEqual(trigger, b.Spec.Triggers[i]) {
			return false
		}
	}
	return true
}

// Ensure the service for the migration registry on the specified cluster has been created
func (t *Task) ensureRegistryService(client k8sclient.Client, registryResources *MigRegistryResources) error {
	registrySecret := registryResources.CredSecret
	if registrySecret == nil {
		return errors.New("migration registry credentials not found")
	}
	name := registrySecret.GetName()
	newService, err := t.buildRegistryService(name)
	if err != nil {
		return err
	}
	foundService, err := t.getRegistryService(client)
	if err != nil {
		return err
	}
	if foundService == nil {
		registryResources.Service = newService
		err = client.Create(context.TODO(), newService)
		if err != nil {
			return err
		}
		return nil
	}
	registryResources.Service = foundService
	if !t.equalsRegistryService(newService, foundService) {
		t.updateRegistryService(foundService, name)
		err = client.Update(context.TODO(), foundService)
		if err != nil {
			return err
		}
	}
	return nil
}

// Build a Registry Service as desired for the specified cluster.
func (t *Task) buildRegistryService(name string) (*kapi.Service, error) {
	labels := t.PlanResources.MigPlan.GetCorrelationLabels()
	labels["app"] = name
	service := &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      name,
			Namespace: VeleroNamespace,
		},
	}
	err := t.updateRegistryService(service, name)
	return service, err
}

// Update a Registry Service as desired for the specified cluster.
func (t *Task) updateRegistryService(service *kapi.Service, name string) error {
	service.Spec = kapi.ServiceSpec{
		Ports: []kapi.ServicePort{
			kapi.ServicePort{
				Name:       "5000-tcp",
				Port:       5000,
				Protocol:   kapi.ProtocolTCP,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 5000},
			},
		},
		Selector: map[string]string{
			"app":              name,
			"deploymentconfig": name,
		},
	}
	return nil
}

// Get an existing registry Service on the specifiedcluster.
func (t *Task) getRegistryService(client k8sclient.Client) (*kapi.Service, error) {
	list := kapi.ServiceList{}
	labels := t.PlanResources.MigPlan.GetCorrelationLabels()
	err := client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Determine if two services are equal.
// Returns `true` when equal.
func (t *Task) equalsRegistryService(a, b *kapi.Service) bool {
	return reflect.DeepEqual(a.Spec.Ports, b.Spec.Ports) &&
		reflect.DeepEqual(a.Spec.Selector, b.Spec.Selector)
}

// Returns the right backup/restore annotations including registry-specific ones
func (t *Task) getAnnotations(registryResources *MigRegistryResources) (map[string]string, error) {
	annotations := t.Annotations
	if len(registryResources.Service.Spec.Ports) == 0 {
		return nil, errors.New("Migration Registry service port not found")
	}
	annotations[MigRegistryAnnotationKey] = fmt.Sprintf("%s:%d", registryResources.Service.Spec.ClusterIP,
		registryResources.Service.Spec.Ports[0].Port)
	for _, container := range registryResources.DeploymentConfig.Spec.Template.Spec.Containers {
		for _, envVar := range container.Env {
			if envVar.Name == "REGISTRY_STORAGE_S3_ROOTDIRECTORY" {
				annotations[MigRegistryDirAnnotationKey] = envVar.Value
			}
		}
	}
	return annotations, nil
}
