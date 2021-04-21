package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	v1 "k8s.io/api/core/v1"
)

const (
	EventParam = "event"
	EventsRoot = NamespaceRoot + "/events"
	EventRoot  = EventsRoot + "/:" + EventParam
)

//
// Event (route) handler.
type EventHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h EventHandler) AddRoutes(r *gin.Engine) {
	r.GET(EventsRoot, h.List)
	r.GET(EventsRoot+"/", h.List)
	r.GET(EventRoot, h.Get)
}

//
// List all of the Events on a cluster.
func (h EventHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Event{
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
	content := EventList{
		Count: count,
	}
	for _, m := range list {
		r := Event{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific Event on a cluster.
func (h EventHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := model.Event{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param(Ns2Param),
			Name:      ctx.Param(EventParam),
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
	r := Event{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h EventHandler) Link(c *model.Cluster, m *model.Event) string {
	return h.BaseHandler.Link(
		EventRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			EventParam:   m.Name,
		})
}

// Event REST resource
type Event struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *v1.Event `json:"object,omitempty"`
}

//
// Build the resource.
func (r *Event) With(m *model.Event) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Event collection REST resource.
type EventList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Event `json:"resources"`
}
