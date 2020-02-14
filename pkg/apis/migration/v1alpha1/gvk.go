package v1alpha1

// UnsupportedNamespace - namespace, which is noticed
// to contain resources unsupported by the migration
type UnsupportedNamespace struct {
	Name                 string   `json:"name"`
	UnsupportedResources []string `json:"unsupportedResources"`
}
