package web

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/fusor/mig-controller/pkg/controller/discovery/auth"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"github.com/gin-gonic/gin"
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	capi "k8s.io/client-go/kubernetes/typed/core/v1"
	"net/http"
	"strconv"
	"strings"
)

const (
	PodsRoot = NamespaceRoot + "/pods"
	PodRoot  = PodsRoot + "/:pod"
	LogRoot  = PodRoot + "/log"
)

//
// Pod (route) handler.
type PodHandler struct {
	// Base
	BaseHandler
}

//
// Add routes.
func (h PodHandler) AddRoutes(r *gin.Engine) {
	r.GET(PodsRoot, h.List)
	r.GET(PodsRoot+"/", h.List)
	r.GET(PodRoot, h.Get)
}

//
// Prepare the handler to fulfil the request.
// Perform RBAC authorization.
func (h *PodHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.allow(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// RBAC authorization.
func (h *PodHandler) allow(ctx *gin.Context) int {
	allowed, err := h.rbac.Allow(&auth.Request{
		Namespace: ctx.Param("ns2"),
		Resources: []string{
			auth.Pod,
		},
		Verbs: []string{
			auth.LIST,
			auth.GET,
		},
	})
	if err != nil {
		Log.Trace(err)
		return http.StatusInternalServerError
	}
	if !allowed {
		return http.StatusForbidden
	}

	return http.StatusOK
}

//
// Get a specific pod.
func (h PodHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	namespace := ctx.Param("ns2")
	name := ctx.Param("pod")
	pod := model.Pod{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: namespace,
			Name:      name,
		},
	}
	err := pod.Select(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		} else {
			ctx.Status(http.StatusNotFound)
			return
		}
	}
	d := Pod{}
	d.With(&pod, &h.cluster)
	ctx.JSON(http.StatusOK, d)
}

//
// List pods on a cluster in a namespace.
func (h PodHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	ns := model.Namespace{
		Base: model.Base{
			Cluster: h.cluster.PK,
			Name:    ctx.Param("ns2"),
		},
	}
	list, err := ns.PodList(h.container.Db, &h.page)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Pod{}
	for _, m := range list {
		d := Pod{}
		d.With(m, &h.cluster)
		content = append(content, d)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Pod-log (route) handler.
type LogHandler struct {
	// Base
	BaseHandler
}

// Add routes.
func (h LogHandler) AddRoutes(r *gin.Engine) {
	r.GET(LogRoot, h.List)
}

//
// Prepare the handler to fulfil the request.
// Perform RBAC authorization.
func (h *LogHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.allow(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// RBAC authorization.
func (h *LogHandler) allow(ctx *gin.Context) int {
	allowed, err := h.rbac.Allow(&auth.Request{
		Namespace: ctx.Param("ns2"),
		Resources: []string{
			auth.PodLog,
		},
		Verbs: []string{
			auth.GET,
		},
	})
	if err != nil {
		Log.Trace(err)
		return http.StatusInternalServerError
	}
	if !allowed {
		return http.StatusForbidden
	}

	return http.StatusOK
}

//
// Not supported.
func (h LogHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
// List all logs (entries) for a pod (and optional container).
func (h LogHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	namespace := ctx.Param("ns2")
	name := ctx.Param("pod")
	pod := model.Pod{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: namespace,
			Name:      name,
		},
	}
	err := pod.Select(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		} else {
			ctx.Status(http.StatusNotFound)
			return
		}
	}

	h.getLog(ctx, &pod)
}

//
// Get the k8s logs.
func (h *LogHandler) getLog(ctx *gin.Context, pod *model.Pod) {
	options, status := h.buildOptions(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	podClient, status := h.buildClient(pod)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	request := podClient.GetLogs(pod.Name, options)
	stream, err := request.Stream()
	if err != nil {
		stErr, cast := err.(*errors.StatusError)
		if cast {
			ctx.String(int(stErr.ErrStatus.Code), stErr.ErrStatus.Message)
			return
		}
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	h.writeBody(ctx, stream)
	stream.Close()
	ctx.Status(http.StatusOK)
}

//
// Write the json-encoded logs in the response.
func (h *LogHandler) writeBody(ctx *gin.Context, stream io.ReadCloser) {
	ctx.Header("Content-Type", "application/json; charset=utf-8")
	// Begin `[`
	_, err := ctx.Writer.Write([]byte("["))
	if err != nil {
		return
	}
	// Add `line,`
	ln := 0
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		if ln < h.page.Offset {
			continue
		}
		if ln >= h.page.Limit {
			break
		}
		if ln > 0 {
			_, err = ctx.Writer.Write([]byte(","))
			if err != nil {
				return
			}
		}
		line, _ := json.Marshal(scanner.Text())
		_, err = ctx.Writer.Write(line)
		if err != nil {
			return
		}
		ln++
	}
	// End `]`
	_, err = ctx.Writer.Write([]byte("]"))
	if err != nil {
		return
	}
}

//
// Build the k8s log API options based on parameters.
// The `tail` parameter indicates that pagination is relative
// to the last log entry.
func (h *LogHandler) buildOptions(ctx *gin.Context) (*v1.PodLogOptions, int) {
	options := v1.PodLogOptions{}
	container := h.getContainer(ctx)
	if container != "" {
		options.Container = container
	}
	tail, status := h.getTail(ctx)
	if status != http.StatusOK {
		return nil, status
	}
	if tail {
		tail := int64(h.page.Limit)
		options.TailLines = &tail
	}

	return &options, http.StatusOK
}

//
// Build the REST client.
func (h *LogHandler) buildClient(pod *model.Pod) (capi.PodInterface, int) {
	ds, found := h.container.GetDs(&h.cluster)
	if !found {
		return nil, http.StatusNotFound
	}
	client, err := kubernetes.NewForConfig(ds.RestCfg)
	if err != nil {
		return nil, http.StatusInternalServerError
	}
	kapi := client.CoreV1()
	podInt := kapi.Pods(pod.Namespace)
	return podInt, http.StatusOK
}

//
// Get the `tail` parameter.
func (h *LogHandler) getTail(ctx *gin.Context) (bool, int) {
	q := ctx.Request.URL.Query()
	s := q.Get("tail")
	if s == "" {
		return false, http.StatusOK
	}
	tail, err := strconv.ParseBool(s)
	if err != nil {
		return false, http.StatusBadRequest
	}

	return tail, http.StatusOK
}

//
// Get the `container` parameter.
func (h *LogHandler) getContainer(ctx *gin.Context) string {
	q := ctx.Request.URL.Query()
	return q.Get("container")
}

//
// Container REST resource.
type Container struct {
	// Pod k8s name.
	Name string `json:"name"`
	// The URI used to obtain logs.
	Log string `json:"log"`
}

//
// Pod REST resource.
type Pod struct {
	// Pod k8s namespace.
	Namespace string `json:"namespace"`
	// Pod k8s name.
	Name string `json:"name"`
	// List of containers.
	Containers []Container `json:"containers"`
}

//
// Container filter.
type ContainerFilter func(*v1.Container) bool

//
// Update fields using the specified models.
func (p *Pod) With(pod *model.Pod, cluster *model.Cluster, filters ...ContainerFilter) {
	p.Containers = []Container{}
	p.Namespace = pod.Namespace
	p.Name = pod.Name
	path := LogRoot
	path = strings.Replace(path, ":ns1", cluster.Namespace, 1)
	path = strings.Replace(path, ":cluster", cluster.Name, 1)
	path = strings.Replace(path, ":ns2", p.Namespace, 1)
	path = strings.Replace(path, ":pod", p.Name, 1)
	for _, container := range p.filterContainers(pod, filters) {
		lp := fmt.Sprintf("%s?container=%s", path, container.Name)
		p.Containers = append(
			p.Containers, Container{
				Name: container.Name,
				Log:  lp,
			})
	}
}

//
// Get a filtered list of containers.
func (p *Pod) filterContainers(pod *model.Pod, filters []ContainerFilter) []v1.Container {
	list := []v1.Container{}
	v1pod := pod.DecodeDefinition()
	podContainers := v1pod.Spec.Containers
	if podContainers == nil {
		return list
	}
	for _, container := range podContainers {
		excluded := false
		for _, filter := range filters {
			if !filter(&container) {
				excluded = true
				break
			}
		}
		if !excluded {
			list = append(list, container)
		}
	}

	return list
}
