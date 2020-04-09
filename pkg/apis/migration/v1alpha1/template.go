package v1alpha1

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TemplateResources struct {
	Templates []TemplateResource `json:"templates,omitempty"`
}

type TemplateResource struct {
	Resource     string `json:"resource"`
	TemplatePath string `json:"templatePath"`
}

func (t *TemplateResource) GroupKind() schema.GroupKind {
	return schema.ParseGroupKind(t.Resource)
}

func (t *TemplateResource) Path() []string {
	return fromJsonPath(t.TemplatePath)
}

func fromJsonPath(path string) []string {
	path = strings.TrimLeft(path, ".")
	return strings.Split(path, ".")
}
