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

var DefaultTemplates = []TemplateResource{
	{
		Resource:     "cronjob.batch",
		TemplatePath: ".spec.jobTemplate.spec.template",
	},
	{
		Resource:     "deployment.apps",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "deploymentconfig.apps.openshift.io",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "replicationcontroller",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "daemonset.apps",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "statefulset.apps",
		TemplatePath: ".spec.template",
	},
	{
		Resource:     "replicaset.apps",
		TemplatePath: ".spec.template",
	},
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
