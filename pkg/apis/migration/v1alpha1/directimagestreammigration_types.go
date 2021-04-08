/*
Copyright 2020 Red Hat Inc.

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

	liberr "github.com/konveyor/controller/pkg/error"
	imagev1 "github.com/openshift/api/image/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// DirectImageStreamMigrationSpec defines the desired state of DirectImageStreamMigration
type DirectImageStreamMigrationSpec struct {
	SrcMigClusterRef  *kapi.ObjectReference `json:"srcMigClusterRef,omitempty"`
	DestMigClusterRef *kapi.ObjectReference `json:"destMigClusterRef,omitempty"`
	ImageStreamRef    *kapi.ObjectReference `json:"imageStreamRef,omitempty"`

	//  Holds the name of the namespace on destination cluster where imagestreams should be migrated.
	DestNamespace string `json:"destNamespace,omitempty"`
}

// DirectImageStreamMigrationStatus defines the observed state of DirectImageStreamMigration
type DirectImageStreamMigrationStatus struct {
	Conditions     `json:","`
	ObservedDigest string       `json:"observedDigest,omitempty"`
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`
	Phase          string       `json:"phase,omitempty"`
	Itinerary      string       `json:"itinerary,omitempty"`
	Errors         []string     `json:"errors,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectImageStreamMigration is the Schema for the directimagestreammigrations API
// +kubebuilder:resource:path=directimagestreammigrations,shortName=dism
// +k8s:openapi-gen=true
type DirectImageStreamMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DirectImageStreamMigrationSpec   `json:"spec,omitempty"`
	Status DirectImageStreamMigrationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DirectImageStreamMigrationList contains a list of DirectImageStreamMigration
type DirectImageStreamMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DirectImageStreamMigration `json:"items"`
}

func (r *DirectImageStreamMigration) GetSourceCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.SrcMigClusterRef)
}

func (r *DirectImageStreamMigration) GetDestinationCluster(client k8sclient.Client) (*MigCluster, error) {
	return GetCluster(client, r.Spec.DestMigClusterRef)
}

// Get the DirectImageMigration that owns this DirectImageStreamMigration. If not owned, return nil.
func (r *DirectImageStreamMigration) GetDIMforDISM(client k8sclient.Client) (*DirectImageMigration, error) {
	owner := &DirectImageMigration{}
	ownerRefs := r.GetOwnerReferences()
	for _, ownerRef := range ownerRefs {
		if ownerRef.Kind != "DirectImageMigration" {
			continue
		}
		ownerRef := types.NamespacedName{Name: ownerRef.Name, Namespace: r.Namespace}
		err := client.Get(context.TODO(), ownerRef, owner)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		return owner, nil
	}
	return nil, nil
}

// Get the Migration associated with this DirectImageStreamMigration. If not owned, return nil.
func (r *DirectImageStreamMigration) GetMigrationForDISM(client k8sclient.Client) (*MigMigration, error) {
	dim, err := r.GetDIMforDISM(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if dim == nil {
		return nil, nil
	}
	migration, err := dim.GetMigrationForDIM(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return migration, nil
}

func (r *DirectImageStreamMigration) GetImageStream(c k8sclient.Client) (*imagev1.ImageStream, error) {
	cluster, err := r.GetSourceCluster(c)
	if err != nil {
		return nil, err
	}
	if !cluster.Status.IsReady() {
		return nil, errors.New("Source cluster not ready")
	}
	client, err := cluster.GetClient(c)
	if err != nil {
		return nil, err
	}
	return GetImageStream(client, r.Spec.ImageStreamRef)
}

// GetDestinationNamespaces get destination namespace
func (r *DirectImageStreamMigration) GetDestinationNamespace() string {
	if len(r.Spec.DestNamespace) > 0 {
		return r.Spec.DestNamespace
	}
	return r.Spec.ImageStreamRef.Namespace
}

// Add (de-duplicated) errors.
func (r *DirectImageStreamMigration) AddErrors(errors []string) {
	m := map[string]bool{}
	for _, e := range r.Status.Errors {
		m[e] = true
	}
	for _, error := range errors {
		_, found := m[error]
		if !found {
			r.Status.Errors = append(r.Status.Errors, error)
		}
	}
}

// HasErrors will notify about error presence on the DirectImageStreamMigration resource
func (r *DirectImageStreamMigration) HasErrors() bool {
	return len(r.Status.Errors) > 0
}

// HasCompleted gets whether a DirectImageStreamMigration has completed and a list of errors, if any
func (r *DirectImageStreamMigration) HasCompleted() (bool, []string) {
	completed := r.Status.Phase == "Completed"
	reasons := r.Status.Errors
	return completed, reasons
}

func init() {
	SchemeBuilder.Register(&DirectImageStreamMigration{}, &DirectImageStreamMigrationList{})
}
