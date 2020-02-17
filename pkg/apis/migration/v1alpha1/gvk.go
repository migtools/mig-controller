package v1alpha1

// UnsupportedNamespace - namespace, which is noticed
// to contain resources unsupported by the migration
type UnsupportedNamespace struct {
	Name                 string   `json:"name"`
	UnsupportedResources []string `json:"unsupportedResources"`
}

func (p *MigPlanStatus) SelectUnsupported() []string {
	namespaces := []string{}
	for _, ns := range p.UnsupportedNamespaces {
		namespaces = append(namespaces, ns.Name)
	}

	return namespaces
}

func (p *MigPlanStatus) InitUnsupported() {
	p.UnsupportedNamespaces = []UnsupportedNamespace{}
}

func (p *MigPlanStatus) AppendUnsupported(namespace string, resources []string) {
	unsupportedNamespace := UnsupportedNamespace{
		Name:                 namespace,
		UnsupportedResources: resources,
	}
	p.UnsupportedNamespaces = append(p.UnsupportedNamespaces, unsupportedNamespace)
}
