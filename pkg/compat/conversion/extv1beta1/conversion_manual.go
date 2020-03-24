package extv1beta1

import (
	v1 "k8s.io/api/apps/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{}
)

func Convert_v1beta1_DaemonSetSpec_To_v1_DaemonSetSpec(in *v1beta1.DaemonSetSpec, out *v1.DaemonSetSpec, s conversion.Scope) error {
	// ignore in.TemplateGeneration
	return autoConvert_v1beta1_DaemonSetSpec_To_v1_DaemonSetSpec(in, out, s)
}

func Convert_v1beta1_DeploymentSpec_To_v1_DeploymentSpec(in *v1beta1.DeploymentSpec, out *v1.DeploymentSpec, s conversion.Scope) error {
	// ignore in.RollbackTo
	return autoConvert_v1beta1_DeploymentSpec_To_v1_DeploymentSpec(in, out, s)
}
