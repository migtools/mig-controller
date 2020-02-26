package web

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	capi "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
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
	err := pod.Get(h.container.Db)
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
	content := Pod{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Object:    pod.DecodeObject(),
	}

	ctx.JSON(http.StatusOK, content)
}

//
// List pods on a cluster in a namespace.
func (h PodHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Pod{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param("ns2"),
		},
	}
	count, err := collection.Count(db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err := collection.List(
		db,
		model.ListOptions{
			Page: &h.page,
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := PodList{
		Count: count,
	}
	for _, m := range list {
		d := Pod{
			Namespace: m.Namespace,
			Name:      m.Name,
			Object:    m.DecodeObject(),
		}
		content.Items = append(content.Items, d)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Pod-log (route) handler.
type LogHandler struct {
	// Base
	ClusterScoped
}

// Add routes.
func (h LogHandler) AddRoutes(r *gin.Engine) {
	r.GET(LogRoot, h.List)
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
	err := pod.Get(h.container.Db)
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

// Pod REST resource
type Pod struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Raw k8s object.
	Object *v1.Pod `json:"object,omitempty"`
}

//
// Pod collection REST resource.
type PodList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Pod `json:"resources"`
}
