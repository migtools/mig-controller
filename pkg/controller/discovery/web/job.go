package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	batchv1 "k8s.io/api/batch/v1"
)

const (
	JobParam = "job"
	JobsRoot = NamespaceRoot + "/jobs"
	JobRoot  = JobsRoot + "/:" + JobParam
)

// Job (route) handler.
type JobHandler struct {
	// Base
	ClusterScoped
}

// Add routes.
func (h JobHandler) AddRoutes(r *gin.Engine) {
	r.GET(JobsRoot, h.List)
	r.GET(JobsRoot+"/", h.List)
	r.GET(JobRoot, h.Get)
}

// List all of the Jobs on a cluster.
func (h JobHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Job{
		Base: model.Base{
			Cluster: h.cluster.PK,
		},
	}
	count, err := collection.Count(db, model.ListOptions{})
	if err != nil {
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
	content := JobList{
		Count: count,
	}
	for _, m := range list {
		r := Job{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific Job on a cluster.
func (h JobHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := model.Job{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param(Ns2Param),
			Name:      ctx.Param(JobParam),
		},
	}
	err := m.Get(h.container.Db)
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
	r := Job{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h JobHandler) Link(c *model.Cluster, m *model.Job) string {
	return h.BaseHandler.Link(
		JobRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			JobParam:     m.Name,
		})
}

// Job REST resource
type Job struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *batchv1.Job `json:"object,omitempty"`
}

// Build the resource.
func (r *Job) With(m *model.Job) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// Job collection REST resource.
type JobList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Job `json:"resources"`
}
