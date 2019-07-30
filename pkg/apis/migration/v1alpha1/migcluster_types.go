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
	"time"

	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	ocapi "github.com/openshift/api/apps/v1"
	imgapi "github.com/openshift/api/image/v1"
	"k8s.io/api/apps/v1"
	kapi "k8s.io/api/core/v1"
	storageapi "k8s.io/api/storage/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	crapi "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MigClusterSpec defines the desired state of MigCluster
type MigClusterSpec struct {
	IsHostCluster           bool                  `json:"isHostCluster"`
	ClusterRef              *kapi.ObjectReference `json:"clusterRef,omitempty"`
	ServiceAccountSecretRef *kapi.ObjectReference `json:"serviceAccountSecretRef,omitempty"`
	StorageClasses          []StorageClass        `json:"storageClasses,omitempty"`
}

// MigClusterStatus defines the observed state of MigCluster
type MigClusterStatus struct {
	Conditions
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

// Ensure finalizer.
func (r *MigCluster) EnsureFinalizer() bool {
	if !FinalizerEnabled {
		return false
	}
	if r.Finalizers == nil {
		r.Finalizers = []string{}
	}
	for _, f := range r.Finalizers {
		if f == Finalizer {
			return false
		}
	}
	r.Finalizers = append(r.Finalizers, Finalizer)
	return true
}

// Delete finalizer.
func (r *MigCluster) DeleteFinalizer() bool {
	if r.Finalizers == nil {
		r.Finalizers = []string{}
	}
	deleted := false
	kept := []string{}
	for _, f := range r.Finalizers {
		if f == Finalizer {
			deleted = true
			continue
		}
		kept = append(kept, f)
	}
	r.Finalizers = kept
	return deleted
}

// Get the service account secret.
// Returns `nil` when the reference cannot be resolved.
func (m *MigCluster) GetServiceAccountSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return GetSecret(client, m.Spec.ServiceAccountSecretRef)
}

// GetClient get a local or remote client using a MigCluster and an existing client
func (m *MigCluster) GetClient(c k8sclient.Client) (k8sclient.Client, error) {

	// If MigCluster isHostCluster, reuse client
	if m.Spec.IsHostCluster {
		return c, nil
	}
	/*
		TODO: re-enable cache after issue with caching secrets is resolved

		// If RemoteWatch exists for this cluster, reuse the remote manager's client
		remoteWatchMap := remote.GetWatchMap()
		remoteWatchCluster := remoteWatchMap.Get(types.NamespacedName{Namespace: m.Namespace, Name: m.Name})
		if remoteWatchCluster != nil {
			return remoteWatchCluster.RemoteManager.GetClient(), nil
		}
	*/
	// If can't reuse a client from anywhere, build one from scratch without a cache
	restConfig, err := m.BuildRestConfig(c)
	if err != nil {
		return nil, err
	}

	client, err := m.buildClient(restConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// CheckConnection checks if MigCluster client config is valid by creating a client
func (m *MigCluster) CheckConnection(c k8sclient.Client, timeout time.Duration) (bool, error) {
	// If MigCluster isHostCluster, no need to connection check
	if m.Spec.IsHostCluster {
		return true, nil
	}

	// If MigCluster is remote, build a client to check connection
	restConfig, err := m.BuildRestConfig(c)
	if err != nil {
		return false, err
	}

	// Allow setting of custom timeout for connection checking
	restConfig.Timeout = timeout

	_, err = m.buildClient(restConfig)
	if err != nil {
		return false, err
	}

	return true, nil
}

// BuildRestConfig creates a remote cluster RestConfig from a MigCluster and a local client
func (m *MigCluster) BuildRestConfig(c k8sclient.Client) (*rest.Config, error) {
	// Get first K8s endpoint from ClusterRef
	clusterRef := m.Spec.ClusterRef
	cluster := &crapi.Cluster{}
	if clusterRef == nil {
		err := errors.New("ClusterRef is nil.")
		return nil, err
	}

	err := c.Get(context.TODO(), types.NamespacedName{Name: clusterRef.Name, Namespace: clusterRef.Namespace}, cluster)
	if err != nil {
		return nil, err // TODO: introspect error type
	}

	if cluster.Spec.KubernetesAPIEndpoints.ServerEndpoints == nil {
		return nil, fmt.Errorf("MigCluster [%s/%s] references Cluster [%s/%s] with nil ServerEndpoints",
			m.Namespace, m.Name, clusterRef.Namespace, clusterRef.Name)
	}

	k8sEndpoints := cluster.Spec.KubernetesAPIEndpoints.ServerEndpoints
	if len(k8sEndpoints) < 1 {
		return nil, fmt.Errorf("MigCluster [%s/%s] references Cluster [%s/%s] with 0 ServerEndpoints",
			m.Namespace, m.Name, clusterRef.Namespace, clusterRef.Name)
	}

	if k8sEndpoints[0].ServerAddress == "" {
		return nil, fmt.Errorf("MigCluster [%s/%s] references Cluster [%s/%s] with an empty ServerAddress at ServerEndpoints[0]",
			m.Namespace, m.Name, clusterRef.Namespace, clusterRef.Name)
	}
	clusterURL := string(k8sEndpoints[0].ServerAddress)

	// Get SA token attached to this MigCluster
	saSecretRef := m.Spec.ServiceAccountSecretRef
	saSecret := &kapi.Secret{}
	if saSecretRef == nil {
		err := errors.New("ServiceAccountSecretRef not set.")
		return nil, err
	}

	err = c.Get(context.TODO(), types.NamespacedName{Name: saSecretRef.Name, Namespace: saSecretRef.Namespace}, saSecret)
	if err != nil {
		return nil, err // TODO: introspect error type
	}

	saTokenKey := "saToken"
	saTokenData, ok := saSecret.Data[saTokenKey]
	if !ok {
		return nil, fmt.Errorf("MigCluster [%s/%s] references SA token secret [%s/%s] with empty 'saToken' field",
			m.Namespace, m.Name, saSecretRef.Namespace, saSecretRef.Name)
	}
	saToken := string(saTokenData)

	// Build insecure rest.Config from gathered data
	// TODO: get caBundle from MigCluster and use that to construct rest.Config
	restConfig := m.buildRestConfig(clusterURL, saToken)

	return restConfig, nil
}

// buildRestConfig creates an insecure REST config from a clusterURL and bearerToken
// TODO: add support for creating a secure rest.Config
func (m *MigCluster) buildRestConfig(clusterURL string, bearerToken string) *rest.Config {
	clusterConfig := &rest.Config{
		Host:        clusterURL,
		BearerToken: bearerToken,
	}
	clusterConfig.Insecure = true
	return clusterConfig
}

// buildClient builds a controller-runtime client for interacting with
// a K8s cluster.
func (m *MigCluster) buildClient(config *rest.Config) (k8sclient.Client, error) {
	c, err := k8sclient.New(config, k8sclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	return c, nil
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
	dList := v1.DeploymentList{}
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
