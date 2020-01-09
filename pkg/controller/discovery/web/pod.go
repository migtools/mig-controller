package web

import (
	"bufio"
	"database/sql"
	"encoding/json"
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
)

//
// Pod (route) handler.
type PodHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h PodHandler) AddRoutes(r *gin.Engine) {
	r.GET(PodsRoot, h.List)
	r.GET(PodsRoot+"/", h.List)
	r.GET(PodRoot, h.Get)
}

//
// Get a specific pod.
func (h PodHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		h.ctx.Status(status)
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
			h.ctx.Status(http.StatusInternalServerError)
			return
		} else {
			h.ctx.Status(http.StatusNotFound)
			return
		}
	}
	d := Pod{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Log:       ctx.Request.URL.Path + "/log",
	}

	h.ctx.JSON(http.StatusOK, d)
}

//
// List pods on a cluster in a namespace.
func (h PodHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		h.ctx.Status(status)
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
		h.ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Pod{}
	for _, m := range list {
		href := strings.TrimSuffix(
			ctx.Request.URL.Path+"/"+m.Name,
			"/") + "/log"
		d := Pod{
			Namespace: m.Namespace,
			Name:      m.Name,
			Log:       href,
		}
		content = append(content, d)
	}

	h.ctx.JSON(http.StatusOK, content)
}

//
// Pod-log (route) handler.
type LogHandler struct {
	// Base
	ClusterScoped
}

// Add routes.
func (h LogHandler) AddRoutes(r *gin.Engine) {
	r.GET(PodRoot+"/log", h.List)
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
		h.ctx.Status(status)
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
			h.ctx.Status(http.StatusInternalServerError)
			return
		} else {
			h.ctx.Status(http.StatusNotFound)
			return
		}
	}

	h.getLog(&pod)
}

//
// Get the k8s logs.
func (h *LogHandler) getLog(pod *model.Pod) {
	options, status := h.buildOptions()
	if status != http.StatusOK {
		h.ctx.Status(status)
		return
	}
	podClient, status := h.buildClient(pod)
	if status != http.StatusOK {
		h.ctx.Status(status)
		return
	}
	request := podClient.GetLogs(pod.Name, options)
	stream, err := request.Stream()
	if err != nil {
		stErr, cast := err.(*errors.StatusError)
		if cast {
			h.ctx.String(int(stErr.ErrStatus.Code), stErr.ErrStatus.Message)
			return
		}
		Log.Trace(err)
		h.ctx.Status(http.StatusInternalServerError)
		return
	}
	h.writeBody(stream)
	stream.Close()
	h.ctx.Status(http.StatusOK)
}

//
// Write the json-encoded logs in the response.
func (h *LogHandler) writeBody(stream io.ReadCloser) {
	h.ctx.Header("Content-Type", "application/json; charset=utf-8")
	// Begin `[`
	_, err := h.ctx.Writer.Write([]byte("["))
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
			_, err = h.ctx.Writer.Write([]byte(","))
			if err != nil {
				return
			}
		}
		line, _ := json.Marshal(scanner.Text())
		_, err = h.ctx.Writer.Write(line)
		if err != nil {
			return
		}
		ln++
	}
	// End `]`
	_, err = h.ctx.Writer.Write([]byte("]"))
	if err != nil {
		return
	}
}

//
// Build the k8s log API options based on parameters.
// The `tail` parameter indicates that pagination is relative
// to the last log entry.
func (h *LogHandler) buildOptions() (*v1.PodLogOptions, int) {
	options := v1.PodLogOptions{}
	container := h.getContainer()
	if container != "" {
		options.Container = container
	}
	tail, status := h.getTail()
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
func (h *LogHandler) getTail() (bool, int) {
	q := h.ctx.Request.URL.Query()
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
func (h *LogHandler) getContainer() string {
	q := h.ctx.Request.URL.Query()
	return q.Get("container")
}

//
// Pod REST resource.
type Pod struct {
	// Pod k8s namespace.
	Namespace string `json:"namespace"`
	// Pod k8s name.
	Name string `json:"name"`
	// A log entry.
	Log string `json:"log"`
}
