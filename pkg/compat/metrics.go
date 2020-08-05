package compat

import (
	"runtime"
	"strings"

	ref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	api "k8s.io/apimachinery/pkg/runtime"
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
// Numeric metrics consts
const (
	one         = 1
	nanoToMilli = 1e6
)

//
// Global reporter.
var Metrics *Reporter

func init() {
	Metrics = &Reporter{
		callCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mtc_client_request_count",
				Help: "MTC client API request counts.",
			},
			[]string{
				Cluster,
				Component,
				Function,
				Kind,
				Method,
			}),
		elapsedCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mtc_client_request_elapsed",
				Help: "MTC client API request cumulative elapsed time.",
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
	callCounter    *prometheus.CounterVec
	elapsedCounter *prometheus.CounterVec
}

//
// Report `get` API call.
func (m *Reporter) Get(client Client, object api.Object, elapsed float64) {
	m.report(client, Get, object, elapsed)
}

//
// Report `list` API call.
func (m *Reporter) List(client Client, object api.Object, elapsed float64) {
	m.report(client, List, object, elapsed)
}

//
// Report `create` API call.
func (m *Reporter) Create(client Client, object api.Object, elapsed float64) {
	m.report(client, Create, object, elapsed)
}

//
// Report `update` API call.
func (m *Reporter) Update(client Client, object api.Object, elapsed float64) {
	m.report(client, Update, object, elapsed)
}

//
// Report `delete` API call.
func (m *Reporter) Delete(client Client, object api.Object, elapsed float64) {
	m.report(client, Delete, object, elapsed)
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
func (m *Reporter) report(client Client, method string, object api.Object, elapsed float64) {
	component, function := m.context()

	m.callCounter.With(
		prometheus.Labels{
			Cluster:   client.RestConfig().Host,
			Component: component,
			Function:  function,
			Kind:      ref.ToKind(object),
			Method:    method,
		}).Add(one)

	m.elapsedCounter.With(
		prometheus.Labels{
			Cluster:   client.RestConfig().Host,
			Component: component,
			Function:  function,
			Kind:      ref.ToKind(object),
			Method:    method,
		}).Add(elapsed)
}
