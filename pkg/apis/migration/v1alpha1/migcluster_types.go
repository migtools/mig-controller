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
	"time"

	pvdr "github.com/konveyor/mig-controller/pkg/cloudprovider"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	ocapi "github.com/openshift/api/apps/v1"
	imgapi "github.com/openshift/api/image/v1"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appv1 "k8s.io/api/apps/v1"
	kapi "k8s.io/api/core/v1"
	storageapi "k8s.io/api/storage/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// SA secret keys.
const (
	SaToken = "saToken"
)

// MigClusterSpec defines the desired state of MigCluster
type MigClusterSpec struct {
	IsHostCluster           bool                  `json:"isHostCluster"`
	URL                     string                `json:"url,omitempty"`
	ServiceAccountSecretRef *kapi.ObjectReference `json:"serviceAccountSecretRef,omitempty"`
	CABundle                []byte                `json:"caBundle,omitempty"`
	StorageClasses          []StorageClass        `json:"storageClasses,omitempty"`
	AzureResourceGroup      string                `json:"azureResourceGroup,omitempty"`
	Insecure                bool                  `json:"insecure,omitempty"`
	RestartRestic           *bool                 `json:"restartRestic,omitempty"`
}

// MigClusterStatus defines the observed state of MigCluster
type MigClusterStatus struct {
	Conditions
	ObservedDigest string `json:"observedDigest,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigCluster is the Schema for the migclusters API
// +k8s:openapi-gen=true
type MigCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigClusterSpec   `json:"spec,omitempty"`
	Status MigClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigClusterList contains a list of MigCluster
type MigClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigCluster `json:"items"`
}

// StorageClass is an available storage class in the cluster
// Name - the storage class name
// Provisioner - the dynamic provisioner for the storage class
// Default - whether or not this storage class is the default
// AccessModes - access modes supported by the dynamic provisioner
type StorageClass struct {
	Name        string                            `json:"name,omitempty"`
	Provisioner string                            `json:"provisioner,omitempty"`
	Default     bool                              `json:"default,omitempty"`
	AccessModes []kapi.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`
}

func init() {
	SchemeBuilder.Register(&MigCluster{}, &MigClusterList{})
}

// Get the service account secret.
// Returns `nil` when the reference cannot be resolved.
func (m *MigCluster) GetServiceAccountSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return GetSecret(client, m.Spec.ServiceAccountSecretRef)
}

// GetClient get a local or remote client using a MigCluster and an existing client
func (m *MigCluster) GetClient(c k8sclient.Client) (compat.Client, error) {
	restConfig, err := m.BuildRestConfig(c)
	if err != nil {
		return nil, err
	}
	client, err := compat.NewClient(restConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Test the connection settings by building a client.
func (m *MigCluster) TestConnection(c k8sclient.Client, timeout time.Duration) error {
	if m.Spec.IsHostCluster {
		return nil
	}
	restConfig, err := m.BuildRestConfig(c)
	if err != nil {
		return err
	}
	restConfig.Timeout = timeout
	_, err = k8sclient.New(restConfig, k8sclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}

	return nil
}

// Build a REST configuration.
func (m *MigCluster) BuildRestConfig(c k8sclient.Client) (*rest.Config, error) {
	if m.Spec.IsHostCluster {
		return config.GetConfig()
	}
	secret, err := GetSecret(c, m.Spec.ServiceAccountSecretRef)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, errors.Errorf("Service Account Secret not found for %v", m.Name)
	}
	var tlsClientConfig rest.TLSClientConfig
	if m.Spec.Insecure {
		tlsClientConfig = rest.TLSClientConfig{Insecure: true}
	} else {
		tlsClientConfig = rest.TLSClientConfig{Insecure: false, CAData: m.Spec.CABundle}
	}
	restConfig := &rest.Config{
		Host:            m.Spec.URL,
		BearerToken:     string(secret.Data[SaToken]),
		TLSClientConfig: tlsClientConfig,
	}

	return restConfig, nil
}

// Delete resources on the cluster by label.
func (m *MigCluster) DeleteResources(client k8sclient.Client, labels map[string]string) error {
	client, err := m.GetClient(client)
	if err != nil {
		return err
	}
	if labels == nil {
		labels = map[string]string{PartOfLabel: Application}
	}

	options := k8sclient.MatchingLabels(labels)

	// Deployment
	dList := appv1.DeploymentList{}
	err = client.List(context.TODO(), options, &dList)
	if err != nil {
		return err
	}
	for _, r := range dList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// DeploymentConfig
	dcList := ocapi.DeploymentConfigList{}
	err = client.List(context.TODO(), options, &dcList)
	if err != nil {
		return err
	}
	for _, r := range dcList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Service
	svList := kapi.ServiceList{}
	err = client.List(context.TODO(), options, &svList)
	if err != nil {
		return err
	}
	for _, r := range svList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Pod
	pList := kapi.PodList{}
	err = client.List(context.TODO(), options, &pList)
	if err != nil {
		return err
	}
	for _, r := range pList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Secret
	sList := kapi.SecretList{}
	err = client.List(context.TODO(), options, &sList)
	if err != nil {
		return err
	}
	for _, r := range sList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// ImageStream
	iList := imgapi.ImageStreamList{}
	err = client.List(context.TODO(), options, &iList)
	if err != nil {
		return err
	}
	for _, r := range iList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Backup
	bList := velero.BackupList{}
	err = client.List(context.TODO(), options, &bList)
	if err != nil {
		return err
	}
	for _, r := range bList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// Restore
	rList := velero.RestoreList{}
	err = client.List(context.TODO(), options, &rList)
	if err != nil {
		return err
	}
	for _, r := range rList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// BSL
	bslList := velero.BackupStorageLocationList{}
	err = client.List(context.TODO(), options, &bslList)
	if err != nil {
		return err
	}
	for _, r := range bslList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	// VSL
	vslList := velero.VolumeSnapshotLocationList{}
	err = client.List(context.TODO(), options, &vslList)
	if err != nil {
		return err
	}
	for _, r := range vslList.Items {
		err = client.Delete(context.TODO(), &r)
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// Get the list of storage classes from the cluster.
func (r *MigCluster) GetStorageClasses(client k8sclient.Client) ([]storageapi.StorageClass, error) {
	list := storageapi.StorageClassList{}
	err := client.List(
		context.TODO(),
		&k8sclient.ListOptions{},
		&list)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *MigCluster) UpdateProvider(provider pvdr.Provider) {
	switch provider.GetName() {
	case pvdr.Azure:
		p, cast := provider.(*pvdr.AzureProvider)
		if cast {
			p.ClusterResourceGroup = r.Spec.AzureResourceGroup
		}
	}
}
