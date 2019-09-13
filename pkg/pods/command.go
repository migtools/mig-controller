package pods

import (
	"bytes"
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Command executed on a Pod.
// RestCfg - The REST configuration for the cluster.
// Pod - The pod on which to execute the command.
// Args - The command (and args) to execute.
// In - An (optional) command input stream.
// Out - The command output stream set by `Run()`.
// Err - the command error stream set by `Run()`.
type PodCommand struct {
	RestCfg *rest.Config
	Pod     *v1.Pod
	Args    []string
	In      io.Reader
	Out     bytes.Buffer
	Err     bytes.Buffer
}

// Run the command.
func (p *PodCommand) Run() error {
	codec := serializer.NewCodecFactory(scheme.Scheme)
	restClient, err := apiutil.RESTClientForGVK(
		schema.GroupVersionKind{
			Version: "v1",
			Kind:    "pods",
		},
		p.RestCfg,
		codec)
	if err != nil {
		return err
	}
	post := restClient.Post().
		Resource("pods").
		Name(p.Pod.Name).
		Namespace(p.Pod.Namespace).
		SubResource("exec")
	post.VersionedParams(
		&v1.PodExecOptions{
			Command: p.Args,
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
		},
		scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(
		p.RestCfg,
		"POST",
		post.URL())
	if err != nil {
		return err
	}
	if p.In == nil {
		p.In = bytes.NewReader([]byte{})
	}
	p.Out = bytes.Buffer{}
	p.Err = bytes.Buffer{}
	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  p.In,
		Stdout: &p.Out,
		Stderr: &p.Err,
		Tty:    false,
	})
	if err != nil {
		return err
	}

	return nil
}
