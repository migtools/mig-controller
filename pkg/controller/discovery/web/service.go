package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	ServiceParam = "service"
	ServicesRoot = NamespaceRoot + "/services"
	ServiceRoot  = ServicesRoot + "/:" + ServiceParam
)

// Service (route) handler.
type ServiceHandler struct {
	// Base
	ClusterScoped
}

// Add routes.
func (h ServiceHandler) AddRoutes(r *gin.Engine) {
	r.GET(ServicesRoot, h.List)
	r.GET(ServicesRoot+"/", h.List)
	r.GET(ServiceRoot, h.Get)
}

// List all of the Services on a cluster.
func (h ServiceHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Service{
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
	content := ServiceList{
		Count: count,
	}
	for _, m := range list {
		r := Service{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific Service on a cluster.
func (h ServiceHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := model.Service{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param(Ns2Param),
			Name:      ctx.Param(ServiceParam),
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
	r := Service{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h ServiceHandler) Link(c *model.Cluster, m *model.Service) string {
	return h.BaseHandler.Link(
		ServiceRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			ServiceParam: m.Name,
		})
}

// Service REST resource
type Service struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *v1.Service `json:"object,omitempty"`
}

// Build the resource.
func (r *Service) With(m *model.Service) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// Service collection REST resource.
type ServiceList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Service `json:"resources"`
}
