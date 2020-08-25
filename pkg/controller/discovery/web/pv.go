package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	PvParam = "pv"
	PvsRoot = ClusterRoot + "/persistentvolumes"
	PvRoot  = PvsRoot + "/:" + PvParam
)

//
// PV (route) handler.
type PvHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h PvHandler) AddRoutes(r *gin.Engine) {
	r.GET(PvsRoot, h.List)
	r.GET(PvsRoot+"/", h.List)
	r.GET(PvRoot, h.Get)
}

//
// List all of the PVs on a cluster.
func (h PvHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.PV{
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
	content := PvList{
		Count: count,
	}
	for _, m := range list {
		r := PV{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific PV on a cluster.
func (h PvHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := model.PV{
		Base: model.Base{
			Cluster: h.cluster.PK,
			Name:    ctx.Param(PvParam),
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
	r := PV{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h PvHandler) Link(c *model.Cluster, m *model.PV) string {
	return h.BaseHandler.Link(
		PvRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			PvParam:      m.Name,
		})
}

// PV REST resource
type PV struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *v1.PersistentVolume `json:"object,omitempty"`
}

//
// Build the resource.
func (r *PV) With(m *model.PV) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// PV collection REST resource.
type PvList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []PV `json:"resources"`
}
