package compat

import (
	ref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	api "k8s.io/apimachinery/pkg/runtime"
	"runtime"
	"strings"
)

//
// Labels.
const (
	Get       = "Get"
	List      = "List"
	Create    = "Create"
	Update    = "Update"
	Delete    = "Delete"
	Cluster   = "cluster"
	Component = "component"
	Function  = "function"
	Kind      = "kind"
	Method    = "method"
)

//
// Global reporter.
var Metrics *Reporter

func init() {
	Metrics = &Reporter{
		counter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mtc_client",
				Help: "MTC client API metrics.",
			},
			[]string{
				Cluster,
				Component,
				Function,
				Kind,
				Method,
			}),
	}
}

//
// Metric reporter.
type Reporter struct {
	counter *prometheus.CounterVec
}

//
// Report `get` API call.
func (m *Reporter) Get(client Client, object api.Object) {
	m.report(client, Get, object)
}

//
// Report `list` API call.
func (m *Reporter) List(client Client, object api.Object) {
	m.report(client, List, object)
}

//
// Report `create` API call.
func (m *Reporter) Create(client Client, object api.Object) {
	m.report(client, Create, object)
}

//
// Report `update` API call.
func (m *Reporter) Update(client Client, object api.Object) {
	m.report(client, Update, object)
}

//
// Report `delete` API call.
func (m *Reporter) Delete(client Client, object api.Object) {
	m.report(client, Delete, object)
}

//
// Determine the call context.
func (m *Reporter) context() (component, function string) {
	bfr := make([]uintptr, 50)
	n := runtime.Callers(5, bfr[:])
	frames := runtime.CallersFrames(bfr[:n])
	for {
		f, hasNext := frames.Next()
		path := strings.Split(f.Function, "/")
		matched := false
		for _, p := range path {
			if p == "controller" {
				matched = true
				continue
			}
			if matched {
				context := strings.Split(p, ".")
				component = context[0]
				function = strings.Join(context[1:], ".")
				break
			}
		}
		if !hasNext {
			break
		}
	}

	return
}

//
// Report the API call.
func (m *Reporter) report(client Client, method string, object api.Object) {
	component, function := m.context()
	m.counter.With(
		prometheus.Labels{
			Cluster:   client.RestConfig().Host,
			Component: component,
			Function:  function,
			Kind:      ref.ToKind(object),
			Method:    method,
		}).Inc()
}
