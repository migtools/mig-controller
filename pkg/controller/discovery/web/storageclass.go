package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	v1 "k8s.io/api/storage/v1"
)

const (
	StorageClassParam = "storageclass"
	StorageClasssRoot = ClusterRoot + "/storageclasses"
	StorageClassRoot  = StorageClasssRoot + "/:" + StorageClassParam
)

//
// StorageClass (route) handler.
type StorageClassHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h StorageClassHandler) AddRoutes(r *gin.Engine) {
	r.GET(StorageClasssRoot, h.List)
	r.GET(StorageClasssRoot+"/", h.List)
	r.GET(StorageClassRoot, h.Get)
}

//
// List all of the StorageClasss on a cluster.
func (h StorageClassHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.StorageClass{
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
	content := StorageClassList{
		Count: count,
	}
	for _, m := range list {
		r := StorageClass{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific StorageClass on a cluster.
func (h StorageClassHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := model.StorageClass{
		Base: model.Base{
			Cluster: h.cluster.PK,
			Name:    ctx.Param(StorageClassParam),
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
	r := StorageClass{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h StorageClassHandler) Link(c *model.Cluster, m *model.StorageClass) string {
	return h.BaseHandler.Link(
		StorageClassRoot,
		Params{
			NsParam:           c.Namespace,
			ClusterParam:      c.Name,
			Ns2Param:          m.Namespace,
			StorageClassParam: m.Name,
		})
}

// StorageClass REST resource
type StorageClass struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *v1.StorageClass `json:"object,omitempty"`
}

//
// Build the resource.
func (r *StorageClass) With(m *model.StorageClass) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// StorageClass collection REST resource.
type StorageClassList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []StorageClass `json:"resources"`
}
