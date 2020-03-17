package v1alpha1

import "k8s.io/apimachinery/pkg/runtime/schema"

// Incompatible - list of namespaces containing incompatible resources for migration
// which are being selected in the MigPlan
type Incompatible struct {
	Namespaces []IncompatibleNamespace `json:"incompatibleNamespaces,omitempty"`
}

// IncompatibleNamespace - namespace, which is noticed
// to contain resources incompatible by the migration
type IncompatibleNamespace struct {
	Name string            `json:"name"`
	GVKs []IncompatibleGVK `json:"gvks"`
}

// IncompatibleGVK - custom structure for printing GVKs lowercase
type IncompatibleGVK struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// FromGVR - allows to convert the scheme.GVR into lowercase IncompatibleGVK
func FromGVR(gvr schema.GroupVersionResource) IncompatibleGVK {
	return IncompatibleGVK{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    gvr.Resource,
	}
}
