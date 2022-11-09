package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	PvcParam = "pvc"
	PvcsRoot = NamespaceRoot + "/persistentvolumeclaims"
	PvcRoot  = PvcsRoot + "/:" + PvcParam
)

// PVC (route) handler.
type PvcHandler struct {
	// Base
	ClusterScoped
}

// Add routes.
func (h PvcHandler) AddRoutes(r *gin.Engine) {
	r.GET(PvcsRoot, h.List)
	r.GET(PvcsRoot+"/", h.List)
	r.GET(PvcRoot, h.Get)
}

// List all of the PVCs on a cluster.
func (h PvcHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.PVC{
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
	content := PvcList{
		Count: count,
	}
	for _, m := range list {
		r := PVC{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific PVC on a cluster.
func (h PvcHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := model.PVC{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param(Ns2Param),
			Name:      ctx.Param(PvcParam),
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
	r := PVC{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h PvcHandler) Link(c *model.Cluster, m *model.PVC) string {
	return h.BaseHandler.Link(
		PvcRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			PvcParam:     m.Name,
		})
}

// PVC REST resource
type PVC struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *v1.PersistentVolumeClaim `json:"object,omitempty"`
}

// Build the resource.
func (r *PVC) With(m *model.PVC) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// PVC collection REST resource.
type PvcList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []PVC `json:"resources"`
}
