/*
Copyright 2019 Red Hat Inc.

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

package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	pvdr "github.com/konveyor/mig-controller/pkg/cloudprovider"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/konveyor/mig-controller/pkg/settings"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appsv1 "k8s.io/api/apps/v1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var Settings = &settings.Settings

// Cache Indexes.
const (
	ClosedIndexField = "closed"
)

// MigPlanHook hold a reference to a MigHook along with the desired phase to run it in
type MigPlanHook struct {
	Reference *kapi.ObjectReference `json:"reference"`

	// Indicates the phase when the hooks will be executed. Acceptable values are: PreBackup, PostBackup, PreRestore, and PostRestore.
	Phase string `json:"phase"`

	// Holds the name of the namespace where hooks should be implemented.
	ExecutionNamespace string `json:"executionNamespace"`

	// Holds the name of the service account to be used for running hooks.
	ServiceAccount string `json:"serviceAccount"`
}

// MigPlanSpec defines the desired state of MigPlan
type MigPlanSpec struct {

	// Holds all the persistent volumes found for the namespaces included in migplan. Each entry is a persistent volume with the information. Name - The PV name. Capacity - The PV storage capacity. StorageClass - The PV storage class name. Supported - Lists of what is supported. Selection - Choices made from supported. PVC - Associated PVC. NFS - NFS properties. staged - A PV has been explicitly added/updated.
	PersistentVolumes `json:",inline"`

	// Holds names of all the namespaces to be included in migration.
	Namespaces []string `json:"namespaces,omitempty"`

	SrcMigClusterRef *kapi.ObjectReference `json:"srcMigClusterRef,omitempty"`

	DestMigClusterRef *kapi.ObjectReference `json:"destMigClusterRef,omitempty"`

	MigStorageRef *kapi.ObjectReference `json:"migStorageRef,omitempty"`

	// If the migration was successful for a migplan, this value can be set True indicating that after one successful migration no new migrations can be carried out for this migplan.
	Closed bool `json:"closed,omitempty"`

	// Holds a reference to a MigHook along with the desired phase to run it in.
	Hooks []MigPlanHook `json:"hooks,omitempty"`

	// If set True, the controller is forced to check if the migplan is in Ready state or not.
	Refresh bool `json:"refresh,omitempty"`

	// If set True, disables direct image migrations.
	IndirectImageMigration bool `json:"indirectImageMigration,omitempty"`

	// If set True, disables direct volume migrations.
	IndirectVolumeMigration bool `json:"indirectVolumeMigration,omitempty"`

	// IncludedResources optional list of included resources in Velero Backup
	// When not set, all the resources are included in the backup
	// +kubebuilder:validation:Optional
	IncludedResources []*metav1.GroupKind `json:"includedResources,omitempty"`

	// LabelSelector optional label selector on the included resources in Velero Backup
	// +kubebuilder:validation:Optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// MigPlanStatus defines the observed state of MigPlan
type MigPlanStatus struct {
	UnhealthyResources `json:",inline"`
	Conditions         `json:",inline"`
	Incompatible       `json:",inline"`
	ObservedDigest     string         `json:"observedDigest,omitempty"`
	ExcludedResources  []string       `json:"excludedResources,omitempty"`
	SrcStorageClasses  []StorageClass `json:"srcStorageClasses,omitempty"`
	DestStorageClasses []StorageClass `json:"destStorageClasses,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigPlan is the Schema for the migplans API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=".spec.srcMigClusterRef.name"
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=".spec.destMigClusterRef.name"
// +kubebuilder:printcolumn:name="Storage",type=string,JSONPath=".spec.migStorageRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MigPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigPlanSpec   `json:"spec,omitempty"`
	Status MigPlanStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigPlanList contains a list of MigPlan
type MigPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigPlan{}, &MigPlanList{})
}

// GetSourceCluster - Get the referenced source cluster.
// Returns `nil` when the reference cannot be resolved.
func (r *MigPlan) GetSourceCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.SrcMigClusterRef)
}

// GetDestinationCluster - Get the referenced destination cluster.
// Returns `nil` when the reference cannot be resolved.
func (r *MigPlan) GetDestinationCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.DestMigClusterRef)
}

// GetStorage - Get the referenced storage.
// Returns `nil` when the reference cannot be resolved.
func (r *MigPlan) GetStorage(client k8sclient.Client) (*MigStorage, error) {
	return GetStorage(client, r.Spec.MigStorageRef)
}

// Resources referenced by the plan.
// Contains all of the fetched referenced resources.
type PlanResources struct {
	MigPlan        *MigPlan
	MigStorage     *MigStorage
	SrcMigCluster  *MigCluster
	DestMigCluster *MigCluster
}

// GetRefResources gets referenced resources from a MigPlan.
func (r *MigPlan) GetRefResources(client k8sclient.Client) (*PlanResources, error) {
	// MigStorage
	storage, err := r.GetStorage(client)
	if err != nil {
		return nil, err
	}
	if storage == nil {
		return nil, errors.New("storage not found")
	}

	// SrcMigCluster
	srcMigCluster, err := r.GetSourceCluster(client)
	if err != nil {
		return nil, err
	}
	if srcMigCluster == nil {
		return nil, errors.New("source cluster not found")
	}

	// DestMigCluster
	destMigCluster, err := r.GetDestinationCluster(client)
	if err != nil {
		return nil, err
	}
	if destMigCluster == nil {
		return nil, errors.New("destination cluster not found")
	}

	resources := &PlanResources{
		MigPlan:        r,
		MigStorage:     storage,
		SrcMigCluster:  srcMigCluster,
		DestMigCluster: destMigCluster,
	}

	return resources, nil
}

// Set the MigPlan Status to closed
func (r *MigPlan) SetClosed() {
	r.Spec.Closed = true
}

// Get list of migrations associated with the plan.
// Sorted by created timestamp with final migrations grouped last.
func (r *MigPlan) ListMigrations(client k8sclient.Client) ([]*MigMigration, error) {
	stage := []*MigMigration{}
	final := []*MigMigration{}
	qlist := MigMigrationList{}

	// Search
	err := client.List(
		context.TODO(),
		&qlist,
		k8sclient.MatchingFields{PlanIndexField: fmt.Sprintf("%s/%s", r.Namespace, r.Name)},
	)
	if err != nil {
		return nil, err
	}
	for i := range qlist.Items {
		migration := qlist.Items[i]
		if migration.Spec.Stage {
			stage = append(stage, &migration)
		} else {
			final = append(final, &migration)
		}
	}

	// Sort by creation timestamp.
	sort.Slice(
		stage,
		func(i, j int) bool {
			a := stage[i].ObjectMeta.CreationTimestamp
			b := stage[j].ObjectMeta.CreationTimestamp
			return a.Before(&b)
		})
	sort.Slice(
		final,
		func(i, j int) bool {
			a := final[i].ObjectMeta.CreationTimestamp
			b := final[j].ObjectMeta.CreationTimestamp
			return a.Before(&b)
		})

	list := append(stage, final...)
	return list, nil
}

//
// Registry
//

// Registry labels for controller-created migration registry resources
const (
	MigrationRegistryLabel = "migration.openshift.io/migration-registry"
)

// Build a credentials Secret as desired for the source cluster.
func (r *MigPlan) BuildRegistrySecret(client k8sclient.Client, storage *MigStorage) (*kapi.Secret, error) {
	labels := r.GetCorrelationLabels()
	labels[MigrationRegistryLabel] = True
	secret := &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       labels,
			GenerateName: "registry-" + string(r.UID) + "-",
			Namespace:    VeleroNamespace,
		},
	}
	return secret, r.UpdateRegistrySecret(client, storage, secret)
}

// Update a Registry credentials secret as desired for the specified cluster.
func (r *MigPlan) UpdateRegistrySecret(client k8sclient.Client, storage *MigStorage, registrySecret *kapi.Secret) error {
	credSecret, err := storage.GetBackupStorageCredSecret(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if credSecret == nil {
		return errors.New("Credentials secret not found.")
	}
	provider := storage.GetBackupStorageProvider()
	err = provider.UpdateRegistrySecret(credSecret, registrySecret)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

// Get an existing credentials Secret on the source cluster.
func (r *MigPlan) GetRegistrySecret(client k8sclient.Client) (*kapi.Secret, error) {
	list := kapi.SecretList{}
	labels := r.GetCorrelationLabels()
	labels[MigrationRegistryLabel] = True
	err := client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(labels))
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
func (r *MigPlan) EqualsRegistrySecret(a, b *kapi.Secret) bool {
	return reflect.DeepEqual(a.Data, b.Data)
}

// Build a Registry Deployment.
func (r *MigPlan) BuildRegistryDeployment(storage *MigStorage, proxySecret *kapi.Secret, name,
	dirName, registryImage string, mCorrelationLabels map[string]string, livenessTimeout int32, readinessTimeout int32) *appsv1.Deployment {
	// Merge correlation labels for plan and migration
	combinedLabels := r.GetCorrelationLabels()
	if mCorrelationLabels != nil {
		for k, v := range mCorrelationLabels {
			combinedLabels[k] = v
		}
	}
	combinedLabels[MigrationRegistryLabel] = True
	combinedLabels["app"] = name
	combinedLabels["migplan"] = string(r.UID)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    combinedLabels,
			Name:      name,
			Namespace: VeleroNamespace,
		},
	}
	r.UpdateRegistryDeployment(storage, deployment, proxySecret, name, dirName, registryImage, mCorrelationLabels, livenessTimeout, readinessTimeout)
	return deployment
}

// Get registry proxy secret for registry DC
// Returns nil if secret isn't found or no configuration exists
func (r *MigPlan) GetProxySecret(client k8sclient.Client) (*kapi.Secret, error) {
	list := kapi.SecretList{}
	selector := k8sLabels.SelectorFromSet(map[string]string{
		"migration-proxy-config": "true",
	})
	err := client.List(
		context.TODO(),
		&list,
		&k8sclient.ListOptions{
			Namespace:     VeleroNamespace,
			LabelSelector: selector,
		},
	)
	// If error listing secrets return no env vars
	if err != nil {
		return nil, err
	}
	// If no secrets found, return nil and no error
	// This means no proxy is configured
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, errors.New("found multiple proxy config secrets")
	}

	return &list.Items[0], nil
}

// Update a Registry Deployment as desired for the specified cluster.
func (r *MigPlan) UpdateRegistryDeployment(storage *MigStorage, deployment *appsv1.Deployment,
	proxySecret *kapi.Secret, name, dirName, registryImage string, mCorrelationLabels map[string]string, livenessTimeout int32, readinessTimeout int32) {

	envFrom := []kapi.EnvFromSource{}
	// If Proxy secret exists, set env from it
	if proxySecret != nil {
		source := kapi.EnvFromSource{
			SecretRef: &kapi.SecretEnvSource{
				LocalObjectReference: kapi.LocalObjectReference{
					Name: proxySecret.Name,
				},
			},
		}
		envFrom = append(envFrom, source)
	}

	// Merge migration correlation labels with Pod labels
	podLabels := map[string]string{
		"app":                  name,
		"deployment":           name,
		"migplan":              string(r.UID),
		MigrationRegistryLabel: True,
	}
	if mCorrelationLabels != nil {
		for k, v := range mCorrelationLabels {
			podLabels[k] = v
		}
	}

	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(1),
		Selector: metav1.SetAsLabelSelector(podLabels),
		Template: kapi.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{},
				Labels:            podLabels,
			},
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					kapi.Container{
						EnvFrom:                  envFrom,
						Image:                    registryImage,
						Name:                     "registry",
						TerminationMessagePolicy: kapi.TerminationMessageFallbackToLogsOnError,
						Ports: []kapi.ContainerPort{
							kapi.ContainerPort{
								ContainerPort: 5000,
								Protocol:      kapi.ProtocolTCP,
							},
						},
						LivenessProbe: &kapi.Probe{
							Handler: kapi.Handler{
								HTTPGet: &kapi.HTTPGetAction{
									Path: "/v2/_catalog?n=5",
									Port: intstr.IntOrString{IntVal: 5000},
								},
							},
							PeriodSeconds:       5,
							TimeoutSeconds:      livenessTimeout,
							InitialDelaySeconds: 15,
						},
						ReadinessProbe: &kapi.Probe{
							Handler: kapi.Handler{
								HTTPGet: &kapi.HTTPGetAction{
									Path: "/v2/_catalog?n=5",
									Port: intstr.IntOrString{IntVal: 5000},
								},
							},
							PeriodSeconds:       5,
							TimeoutSeconds:      readinessTimeout,
							InitialDelaySeconds: 15,
						},
						Resources: kapi.ResourceRequirements{},
						VolumeMounts: []kapi.VolumeMount{
							kapi.VolumeMount{
								MountPath: "/var/lib/registry",
								Name:      "volume-1",
							},
						},
					},
				},
				Volumes: []kapi.Volume{
					kapi.Volume{
						Name:         "volume-1",
						VolumeSource: kapi.VolumeSource{EmptyDir: &kapi.EmptyDirVolumeSource{}},
					},
				},
			},
		},
	}
	provider := storage.GetBackupStorageProvider()
	provider.UpdateRegistryDeployment(deployment, name, dirName)
}

// Get an existing registry Deployment on the specified cluster.
//TODO: We need to convert this from a list call to a get call. The name of the deployment is equal to the name of the secret
func (r *MigPlan) GetRegistryDeployment(client k8sclient.Client) (*appsv1.Deployment, error) {
	list := appsv1.DeploymentList{}
	labels := r.GetCorrelationLabels()
	labels[MigrationRegistryLabel] = True
	err := client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(labels))
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Determine if two deployments are equal.
// Returns `true` when equal.
func (r *MigPlan) EqualsRegistryDeployment(a, b *appsv1.Deployment) bool {
	if !(reflect.DeepEqual(a.Spec.Replicas, b.Spec.Replicas) &&
		reflect.DeepEqual(a.Spec.Selector, b.Spec.Selector) &&
		reflect.DeepEqual(a.Spec.Template.ObjectMeta, b.Spec.Template.ObjectMeta) &&
		reflect.DeepEqual(a.Spec.Template.Spec.Volumes, b.Spec.Template.Spec.Volumes) &&
		len(a.Spec.Template.Spec.Containers) == len(b.Spec.Template.Spec.Containers)) {
		return false
	}
	for i, container := range a.Spec.Template.Spec.Containers {
		if !(reflect.DeepEqual(container.Env, b.Spec.Template.Spec.Containers[i].Env) &&
			reflect.DeepEqual(container.Name, b.Spec.Template.Spec.Containers[i].Name) &&
			reflect.DeepEqual(container.Ports, b.Spec.Template.Spec.Containers[i].Ports) &&
			reflect.DeepEqual(container.LivenessProbe, b.Spec.Template.Spec.Containers[i].LivenessProbe) &&
			reflect.DeepEqual(container.ReadinessProbe, b.Spec.Template.Spec.Containers[i].ReadinessProbe) &&
			reflect.DeepEqual(container.TerminationMessagePolicy, b.Spec.Template.Spec.Containers[i].TerminationMessagePolicy) &&
			reflect.DeepEqual(container.Image, b.Spec.Template.Spec.Containers[i].Image) &&
			reflect.DeepEqual(container.VolumeMounts, b.Spec.Template.Spec.Containers[i].VolumeMounts)) {
			return false
		}
	}
	return true
}

// Build a Registry Service as desired for the specified cluster.
func (r *MigPlan) BuildRegistryService(name string) *kapi.Service {
	labels := r.GetCorrelationLabels()
	labels[MigrationRegistryLabel] = True
	labels["app"] = name
	service := &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Name:      name,
			Namespace: VeleroNamespace,
		},
	}
	r.UpdateRegistryService(service, name)
	return service
}

// Update a Registry Service as desired for the specified cluster.
func (r *MigPlan) UpdateRegistryService(service *kapi.Service, name string) {
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
			"app":        name,
			"deployment": name,
		},
	}
}

// Get an existing registry Service on the specifiedcluster.
//TODO: We need to convert this from a list call to a get call. The name of the deployment is equal to the name of the secret
func (r *MigPlan) GetRegistryService(client k8sclient.Client) (*kapi.Service, error) {
	list := kapi.ServiceList{}
	labels := r.GetCorrelationLabels()
	labels[MigrationRegistryLabel] = True
	err := client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(labels))
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
func (r *MigPlan) EqualsRegistryService(a, b *kapi.Service) bool {
	return reflect.DeepEqual(a.Spec.Ports, b.Spec.Ports) &&
		reflect.DeepEqual(a.Spec.Selector, b.Spec.Selector)
}

//
// Storage
//

// Get an associated BSL by Label search.
// Returns `nil` when not found.
func (r *MigPlan) GetBSL(client k8sclient.Client) (*velero.BackupStorageLocation, error) {
	list := velero.BackupStorageLocationList{}
	labels := r.GetCorrelationLabels()
	err := client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(labels))
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Get an associated VSL by Label search.
// Returns `nil` when not found.
func (r *MigPlan) GetVSL(client k8sclient.Client) (*velero.VolumeSnapshotLocation, error) {
	list := velero.VolumeSnapshotLocationList{}
	labels := r.GetCorrelationLabels()
	err := client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(labels))
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Get the cloud credentials secret by labels for the provider.
func (r *MigPlan) GetCloudSecret(client k8sclient.Client, provider pvdr.Provider) (*kapi.Secret, error) {
	return GetSecret(
		client,
		&kapi.ObjectReference{
			Namespace: VeleroNamespace,
			Name:      provider.GetCloudSecretName(),
		})
}

// GetSourceNamespaces get source namespaces without mapping
func (r *MigPlan) GetSourceNamespaces() []string {
	includedNamespaces := []string{}
	for _, namespace := range r.Spec.Namespaces {
		namespace = strings.Split(namespace, ":")[0]
		includedNamespaces = append(includedNamespaces, namespace)
	}

	return includedNamespaces
}

// GetDestinationNamespaces get destination namespaces without mapping
func (r *MigPlan) GetDestinationNamespaces() []string {
	includedNamespaces := []string{}
	for _, namespace := range r.Spec.Namespaces {
		namespaces := strings.Split(namespace, ":")
		if len(namespaces) > 1 {
			includedNamespaces = append(includedNamespaces, namespaces[1])
		} else {
			includedNamespaces = append(includedNamespaces, namespaces[0])
		}
	}

	return includedNamespaces
}

// GetNamespaceMapping gets a map of src to dest namespaces
func (r *MigPlan) GetNamespaceMapping() map[string]string {
	nsMapping := make(map[string]string)
	for _, namespace := range r.Spec.Namespaces {
		namespaces := strings.Split(namespace, ":")
		if len(namespaces) > 1 {
			nsMapping[namespaces[0]] = namespaces[1]
		} else {
			nsMapping[namespaces[0]] = namespaces[0]
		}
	}

	return nsMapping
}

// Get whether the plan conflicts with another.
// Plans conflict when:
//   - Have any of the clusters in common.
//   - Hand any of the namespaces in common.
func (r *MigPlan) HasConflict(plan *MigPlan) bool {
	if !migref.RefEquals(r.Spec.SrcMigClusterRef, plan.Spec.SrcMigClusterRef) &&
		!migref.RefEquals(r.Spec.DestMigClusterRef, plan.Spec.DestMigClusterRef) &&
		!migref.RefEquals(r.Spec.SrcMigClusterRef, plan.Spec.DestMigClusterRef) &&
		!migref.RefEquals(r.Spec.DestMigClusterRef, plan.Spec.SrcMigClusterRef) {
		return false
	}
	nsMap := map[string]bool{}
	for _, name := range plan.Spec.Namespaces {
		nsMap[name] = true
	}
	for _, name := range r.Spec.Namespaces {
		if _, found := nsMap[name]; found {
			return true
		}
	}

	return false
}

// GetDestinationNamespaces get destination namespaces without mapping
func (r *MigPlan) IsResourceExcluded(resource string) bool {
	for _, excludedResource := range r.Status.ExcludedResources {
		if resource == excludedResource {
			return true
		}
	}
	return false
}

// IsImageMigrationDisabled returns whether this MigPlan has disable_image_copy or disabled_image_migration
// disable_image_copy is a flag available as a controller boolean env var.
// disabled_image_migration is currently implemented site-wide via the ExcludedResources list. This
// This will change to an explicit controller boolean env var at some point.
func (r *MigPlan) IsImageMigrationDisabled() bool {
	return r.IsResourceExcluded(settings.ISResource) || Settings.DisImgCopy
}

// IsVolumeMigrationDisabled returns whether this MigPlan has disabled Volume Migration
// Currently this is only implemented site-wide via the ExcludedResources list. This
// This will change to an explicit controller boolean env var at some point.
func (r *MigPlan) IsVolumeMigrationDisabled() bool {
	return r.IsResourceExcluded(settings.PVResource)
}

//
//
// PV list
//
//

// PV Actions.
const (
	PvMoveAction = "move"
	PvCopyAction = "copy"
	PvSkipAction = "skip"
)

// PV Copy Methods.
const (
	PvFilesystemCopyMethod = "filesystem"
	PvSnapshotCopyMethod   = "snapshot"
)

// Name - The PV name.
// Capacity - The PV storage capacity.
// StorageClass - The PV storage class name.
// Supported - Lists of what is supported.
// Selection - Choices made from supported.
// PVC - Associated PVC.
// NFS - NFS properties.
// staged - A PV has been explicitly added/updated.
type PV struct {
	Name              string                `json:"name,omitempty"`
	Capacity          resource.Quantity     `json:"capacity,omitempty"`
	StorageClass      string                `json:"storageClass,omitempty"`
	Supported         Supported             `json:"supported"`
	Selection         Selection             `json:"selection"`
	PVC               PVC                   `json:"pvc,omitempty"`
	NFS               *kapi.NFSVolumeSource `json:"-"`
	staged            bool                  `json:"-"`
	CapacityConfirmed bool                  `json:"capacityConfirmed,omitempty"`
	ProposedCapacity  resource.Quantity     `json:"proposedCapacity,omitempty"`
}

// PVC
type PVC struct {
	Namespace    string                            `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	Name         string                            `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	AccessModes  []kapi.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`
	HasReference bool                              `json:"hasReference,omitempty"`
}

// GetTargetName returns name of the target PVC
func (p PVC) GetTargetName() string {
	names := strings.Split(p.Name, ":")
	if len(names) > 1 {
		return names[1]
	}
	if len(names) > 0 {
		return names[0]
	}
	return p.Name
}

// GetSourceName returns name of the source PVC
func (p PVC) GetSourceName() string {
	names := strings.Split(p.Name, ":")
	if len(names) > 0 {
		return names[0]
	}
	return p.Name
}

// Supported
// Actions     - The list of supported actions
// CopyMethods - The list of supported copy methods
type Supported struct {
	Actions     []string `json:"actions"`
	CopyMethods []string `json:"copyMethods"`
}

// Selection
// Action - The PV migration action (move|copy|skip)
// StorageClass - The PV storage class name to use in the destination cluster.
// AccessMode   - The PV access mode to use in the destination cluster, if different from src PVC AccessMode
// CopyMethod   - The PV copy method to use ('filesystem' for restic copy, or 'snapshot' for velero snapshot plugin)
// Verify       - Whether or not to verify copied volume data if CopyMethod is 'filesystem'
type Selection struct {
	Action       string                          `json:"action,omitempty"`
	StorageClass string                          `json:"storageClass,omitempty"`
	AccessMode   kapi.PersistentVolumeAccessMode `json:"accessMode,omitempty" protobuf:"bytes,1,rep,name=accessMode,casttype=PersistentVolumeAccessMode"`
	CopyMethod   string                          `json:"copyMethod,omitempty"`
	Verify       bool                            `json:"verify,omitempty"`
}

// Update the PV with another.
func (r *PV) Update(pv PV) {
	r.StorageClass = pv.StorageClass
	r.Supported.Actions = pv.Supported.Actions
	r.Supported.CopyMethods = pv.Supported.CopyMethods
	r.Capacity = pv.Capacity
	r.PVC = pv.PVC
	r.NFS = pv.NFS
	if len(r.Supported.Actions) == 1 {
		r.Selection.Action = r.Supported.Actions[0]
	}
	r.staged = true
}

// Collection of PVs
// List - The collection of PVs.
// index - List index.
// --------
// Example:
// plan.Spec.BeginPvStaging()
// plan.Spec.AddPv(pvA)
// plan.Spec.AddPv(pvB)
// plan.Spec.AddPv(pvC)
// plan.Spec.EndPvStaging()
//
type PersistentVolumes struct {
	List    []PV           `json:"persistentVolumes,omitempty"`
	index   map[string]int `json:"-"`
	staging bool           `json:"-"`
}

// Allocate collections.
func (r *PersistentVolumes) init() {
	if r.List == nil {
		r.List = []PV{}
	}
	if r.index == nil {
		r.buildIndex()
	}
}

// Build the index.
func (r *PersistentVolumes) buildIndex() {
	r.index = map[string]int{}
	for i, pv := range r.List {
		r.index[pv.Name] = i
	}
}

// Begin staging PVs.
func (r *PersistentVolumes) BeginPvStaging() {
	r.init()
	r.staging = true
	for i := range r.List {
		pv := &r.List[i]
		pv.staged = false
	}
}

// End staging PVs and delete un-staged PVs from the list.
// The PVs are sorted by Name.
func (r *PersistentVolumes) EndPvStaging() {
	r.init()
	r.staging = false
	kept := []PV{}
	for _, pv := range r.List {
		if pv.staged {
			kept = append(kept, pv)
		}
	}
	sort.Slice(
		kept,
		func(i, j int) bool {
			return kept[i].Name < kept[j].Name
		})
	r.List = kept
	r.buildIndex()
}

// Find a PV
func (r *PersistentVolumes) FindPv(pv PV) *PV {
	r.init()
	i, found := r.index[pv.Name]
	if found {
		return &r.List[i]
	}

	return nil
}

// Find a PVC
func (r *PersistentVolumes) FindPVC(namespace string, name string) *PVC {
	for i := range r.List {
		pv := &r.List[i]
		if pv.PVC.GetSourceName() == name && pv.PVC.Namespace == namespace {
			return &pv.PVC
		}
	}
	return nil
}

// Add (or update) Pv to the collection.
func (r *PersistentVolumes) AddPv(pv PV) {
	r.init()
	pv.staged = true
	foundPv := r.FindPv(pv)
	if foundPv == nil {
		r.List = append(r.List, pv)
		r.index[pv.Name] = len(r.List) - 1
	} else {
		foundPv.Update(pv)
	}
}

// Delete a PV from the collection.
func (r *PersistentVolumes) DeletePv(names ...string) {
	r.init()
	for _, name := range names {
		i, found := r.index[name]
		if found {
			r.List = append(r.List[:i], r.List[i+1:]...)
			r.buildIndex()
		}
	}
}

// Reset PVs collection.
func (r *PersistentVolumes) ResetPvs() {
	r.init()
	r.List = []PV{}
	r.buildIndex()
}

// Convert name to a DNS_LABEL-compliant string
// DNS_LABEL:  This is a string, no more than 63 characters long, that conforms
//     to the definition of a "label" in RFCs 1035 and 1123. This is captured
//     by the following regex:
//         [a-z0-9]([-a-z0-9]*[a-z0-9])?
func toDnsLabel(name string) (string, error) {
	// keep lowercase alphanumeric and hyphen
	reg, err := regexp.Compile("[^-a-z0-9]+")
	if err != nil {
		return "", err
	}
	processedName := reg.ReplaceAllString(strings.ToLower(name), "")
	// strip initial and trailing hyphens
	stripEndsReg, err := regexp.Compile("(^-+|-+$)")
	if err != nil {
		return "", err
	}
	endsStripped := stripEndsReg.ReplaceAllString(processedName, "")
	return endsStripped, nil
}
