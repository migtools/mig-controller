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
	"fmt"
	kapi "k8s.io/api/core/v1"
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

func init() {
	SchemeBuilder.Register(&MigCluster{}, &MigClusterList{})
}

// Get the service account secret.
// Returns `nil` when the reference cannot be resolved.
func (m *MigCluster) GetServiceAccountSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return GetSecret(client, m.Spec.ServiceAccountSecretRef)
}

// GetClient get a local or remote client using a MigCluster and an existing client
func (m *MigCluster) GetClient(c k8sclient.Client) (k8sclient.Client, error) {
	if m.Spec.IsHostCluster {
		return c, nil
	}
	restConfig, err := m.BuildRestConfig(c)
	if err != nil {
		return nil, err
	}

	client, err := buildClient(restConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// BuildRestConfig creates a remote cluster RestConfig from a MigCluster and a local client
func (m *MigCluster) BuildRestConfig(c k8sclient.Client) (*rest.Config, error) {
	// Get first K8s endpoint from ClusterRef
	clusterRef := m.Spec.ClusterRef
	cluster := &crapi.Cluster{}

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
	restConfig := buildRestConfig(clusterURL, saToken)

	return restConfig, nil
}

// buildRestConfig creates an insecure REST config from a clusterURL and bearerToken
// TODO: add support for creating a secure rest.Config
func buildRestConfig(clusterURL string, bearerToken string) *rest.Config {
	clusterConfig := &rest.Config{
		Host:        clusterURL,
		BearerToken: bearerToken,
	}
	clusterConfig.Insecure = true
	return clusterConfig
}

// buildClient builds a controller-runtime client for interacting with
// a K8s cluster.
func buildClient(config *rest.Config) (k8sclient.Client, error) {
	c, err := k8sclient.New(config, k8sclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	return c, nil
}
