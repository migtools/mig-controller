package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// Restore route root.
const (
	RestoreParam = "restore"
	RestoresRoot = NamespaceRoot + "/restores"
	RestoreRoot  = RestoresRoot + "/:" + RestoreParam
)

// Restore (route) handler.
type RestoreHandler struct {
	// Base
	ClusterScoped
	// Restore referenced in the request.
	restore model.Restore
}

// Add routes.
func (h RestoreHandler) AddRoutes(r *gin.Engine) {
	r.GET(RestoresRoot, h.List)
	r.GET(RestoresRoot+"/", h.List)
	r.GET(RestoreRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced restore.
func (h *RestoreHandler) Prepare(ctx *gin.Context) int {
	status := h.ClusterScoped.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(RestoreParam)
	if name != "" {
		h.restore = model.Restore{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(RestoreParam),
			},
		}
		err := h.restore.Get(h.container.Db)
		if err != nil {
			if err != sql.ErrNoRows {
				sink.Trace(err)
				return http.StatusInternalServerError
			} else {
				return http.StatusNotFound
			}
		}
	}

	return http.StatusOK
}

// List all of the restores in the namespace.
func (h RestoreHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Restore{}
	count, err := collection.Count(db, model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err := collection.List(db, model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := RestoreList{
		Count: count,
	}
	for _, m := range list {
		r := Restore{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific restore.
func (h RestoreHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.restore.Get(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			sink.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		} else {
			ctx.Status(http.StatusNotFound)
			return
		}
	}
	r := Restore{}
	r.With(&h.restore)
	r.SelfLink = h.Link(&h.cluster, &h.restore)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h RestoreHandler) Link(c *model.Cluster, m *model.Restore) string {
	return h.BaseHandler.Link(
		RestoreRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			RestoreParam: m.Name,
		})
}

// Restore REST resource.
type Restore struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Raw k8s object.
	Object *velero.Restore `json:"object,omitempty"`
	// Self URI.
	SelfLink string `json:"selfLink"`
}

// Build the resource.
func (r *Restore) With(m *model.Restore) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// Restore collection REST resource.
type RestoreList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Restore `json:"resources"`
}
