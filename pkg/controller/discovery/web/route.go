package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	v1 "github.com/openshift/api/route/v1"
)

const (
	RouteParam = "route"
	RoutesRoot = NamespaceRoot + "/routes"
	RouteRoot  = RoutesRoot + "/:" + RouteParam
)

//
// Route (route) handler.
type RouteHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h RouteHandler) AddRoutes(r *gin.Engine) {
	r.GET(RoutesRoot, h.List)
	r.GET(RoutesRoot+"/", h.List)
	r.GET(RouteRoot, h.Get)
}

//
// Get a specific route.
func (h RouteHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	namespace := ctx.Param(Ns2Param)
	name := ctx.Param(RouteParam)
	m := model.Route{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: namespace,
			Name:      name,
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
	r := Route{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// List routes on a cluster in a namespace.
func (h RouteHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Route{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param(Ns2Param),
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
	content := RouteList{
		Count: count,
	}
	for _, m := range list {
		r := Route{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h RouteHandler) Link(c *model.Cluster, m *model.Route) string {
	return h.BaseHandler.Link(
		RouteRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			RouteParam:   m.Name,
		})
}

// Route REST resource
type Route struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *v1.Route `json:"object,omitempty"`
}

//
// Build the resource.
func (r *Route) With(m *model.Route) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Route collection REST resource.
type RouteList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Route `json:"resources"`
}
