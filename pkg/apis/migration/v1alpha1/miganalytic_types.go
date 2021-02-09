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
	"k8s.io/apimachinery/pkg/api/resource"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigAnalyticSpec defines the desired state of MigAnalytic
type MigAnalyticSpec struct {
	MigPlanRef *kapi.ObjectReference `json:"migPlanRef"`

	// Enables analysis of persistent volume capacity, if set true. This is a required field.
	AnalyzePVCapacity bool `json:"analyzePVCapacity"`

	// Enables analysis of image count, if set true. This is a required field.
	AnalyzeImageCount bool `json:"analyzeImageCount"`

	// Enables analysis of k8s resources, if set true. This is a required field.
	AnalyzeK8SResources bool `json:"analyzeK8SResources"`

	// Enable used in analysis of image count, if set true.
	ListImages bool `json:"listImages,omitempty"`

	// Represents limit on image counts
	ListImagesLimit int `json:"listImagesLimit,omitempty"`

	// Enables advanced analysis of volumes required for PV resizing
	AnalyzeExntendedPVCapacity bool `json:"analyzeExtendedPVCapacity,omitempty"`

	// Enables refreshing existing MigAnalytic
	Refresh bool `json:"refresh,omitempty"`
}

// MigAnalyticStatus defines the observed state of MigAnalytic
type MigAnalyticStatus struct {
	Conditions         `json:","`
	ObservedGeneration int64           `json:"observedGeneration,omitempty"`
	Analytics          MigAnalyticPlan `json:"analytics,omitempty"`
}

// MigAnalyticPlan defines the observed state of MigAnalyticPlan
type MigAnalyticPlan struct {
	Plan                         string                 `json:"plan"`
	PercentComplete              int                    `json:"percentComplete"`
	K8SResourceTotal             int                    `json:"k8sResourceTotal"`
	ExcludedK8SResourceTotal     int                    `json:"excludedk8sResourceTotal"`
	IncompatibleK8SResourceTotal int                    `json:"incompatiblek8sResourceTotal"`
	PVCapacity                   resource.Quantity      `json:"pvCapacity"`
	PVCount                      int                    `json:"pvCount"`
	ImageCount                   int                    `json:"imageCount"`
	ImageSizeTotal               resource.Quantity      `json:"imageSizeTotal"`
	Namespaces                   []MigAnalyticNamespace `json:"namespaces,omitempty"`
}

// MigAnalyticNamespace defines the observed state of MigAnalyticNamespace
type MigAnalyticNamespace struct {
	Namespace                    string                             `json:"namespace"`
	K8SResourceTotal             int                                `json:"k8sResourceTotal"`
	ExcludedK8SResourceTotal     int                                `json:"excludedK8SResourceTotal"`
	IncompatibleK8SResourceTotal int                                `json:"incompatibleK8SResourceTotal"`
	PVCapacity                   resource.Quantity                  `json:"pvCapacity"`
	PVCount                      int                                `json:"pvCount"`
	ImageCount                   int                                `json:"imageCount"`
	ImageSizeTotal               resource.Quantity                  `json:"imageSizeTotal"`
	Images                       []MigAnalyticNSImage               `json:"images,omitempty"`
	K8SResources                 []MigAnalyticNSResource            `json:"k8sResources,omitempty"`
	PersistentVolumes            []MigAnalyticPersistentVolumeClaim `json:"persistentVolumes,omitempty"`
	ExcludedK8SResources         []MigAnalyticNSResource            `json:"excludedK8SResources,omitempty"`
	IncompatibleK8SResources     []MigAnalyticNSResource            `json:"incompatibleK8SResources,omitempty"`
}

// MigAnalyticNamespaceResource defines the observed state of MigAnalyticNamespaceResource
type MigAnalyticNSResource struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
	Count   int    `json:"count"`
}

type MigAnalyticNSImage struct {
	Name      string            `json:"name"`
	Reference string            `json:"reference"`
	Size      resource.Quantity `json:"size"`
}

// MigAnalyticPersistentVolumeClaim represents a Kubernetes Persistent volume claim with discovered analytic properties
type MigAnalyticPersistentVolumeClaim struct {
	// Name of the persistent volume claim
	Name string `json:"name"`
	// Requested capacity of the claim
	RequestedCapacity resource.Quantity `json:"requestedCapacity,omitempty"`
	// Actual provisioned capacity of the volume
	ActualCapacity resource.Quantity `json:"actualCapacity,omitempty"`
	// Usage of volume in percentage
	UsagePercentage int `json:"usagePercentage,omitempty"`
	// Adjusted capacity of the volume
	ProposedCapacity resource.Quantity `json:"proposedCapacity,omitempty"`
	// Human readable reason for proposed adjustment
	Comment string `json:"comment,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigAnalytic is the Schema for the miganalytics API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=".spec.migPlanRef.name"
// +kubebuilder:printcolumn:name="Progress",type=string,JSONPath=".status.analytics.percentComplete"
// +kubebuilder:printcolumn:name="Resources",type=string,JSONPath=".status.analytics.k8sResourceTotal"
// +kubebuilder:printcolumn:name="Images",type=string,JSONPath=".status.analytics.imageCount"
// +kubebuilder:printcolumn:name="ImageSize",type=string,JSONPath=".status.analytics.imageSizeTotal"
// +kubebuilder:printcolumn:name="PVs",type=string,JSONPath=".status.analytics.pvCount"
// +kubebuilder:printcolumn:name="PVCapacity",type=string,JSONPath=".status.analytics.pvCapacity"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MigAnalytic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MigAnalyticSpec   `json:"spec,omitempty"`
	Status            MigAnalyticStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigAnalyticList contains a list of MigAnalytic
type MigAnalyticList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigAnalytic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigAnalytic{}, &MigAnalyticList{})
}
