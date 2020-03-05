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
	GVRs []IncompatibleGVR `json:"gvrs"`
}

// IncompatibleGVR - custom sructure for printing GVRs lowercase
type IncompatibleGVR struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

// FromGVR - allows to convert the scheme.GVR into lowercase IncompatibleGVR
func FromGVR(gvr schema.GroupVersionResource) IncompatibleGVR {
	return IncompatibleGVR{
		Group:    gvr.Group,
		Version:  gvr.Version,
		Resource: gvr.Resource,
	}
}
