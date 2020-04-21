package v1alpha1

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TemplateResources - list of additional custom resources,
// which contain pod template spec for pod replication.
// Could be extended with user-specified values.
// Formatting is as follows:
//
// templates:
// - resource:     "cronjob.batch",
// 	 templatePath: ".spec.jobTemplate.spec.template",
// - resource:     "deployment.apps",
// 	 templatePath: ".spec.template",
type TemplateResources struct {
	Templates []TemplateResource `json:"templates,omitempty"`
}

// TemplateResource - contains nessesary information to access
// pod template spec form on the resource, used to create Stage pods
type TemplateResource struct {
	Resource     string `json:"resource"`
	TemplatePath string `json:"templatePath"`
}

// DefaultTemplates - list of default resources, known to contain
// pod template specs, used to create stage pod templates
var DefaultTemplates = []TemplateResource{
	{
		Resource:     "job.batch",
		TemplatePath: ".spec.template",
	},
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

// GroupKind - convert resource to a group kind
func (t *TemplateResource) GroupKind() schema.GroupKind {
	return schema.ParseGroupKind(t.Resource)
}

// Path - JSON path to template in the resource spec
func (t *TemplateResource) Path() []string {
	return fromJSONPath(t.TemplatePath)
}

func fromJSONPath(path string) []string {
	path = strings.TrimLeft(path, ".")
	return strings.Split(path, ".")
}
