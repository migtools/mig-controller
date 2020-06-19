package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/auth"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	ServicesRoot = NamespaceRoot + "/services"
	ServiceRoot  = ServicesRoot + "/:service"
)

//
// Service (route) handler.
type ServiceHandler struct {
	// Base
	BaseHandler
}

//
// Add routes.
func (h ServiceHandler) AddRoutes(r *gin.Engine) {
	r.GET(ServicesRoot, h.List)
	r.GET(ServicesRoot+"/", h.List)
	r.GET(ServiceRoot, h.Get)
}

//
// Prepare the handler to fulfil the request.
// Perform RBAC authorization.
func (h *ServiceHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.allow(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// RBAC authorization.
func (h *ServiceHandler) allow(ctx *gin.Context) int {
	allowed, err := h.rbac.Allow(&auth.Review{
		Namespace: ctx.Param("ns2"),
		Resource:  auth.Service,
		Verb:      auth.GET,
	})
	if err != nil {
		Log.Trace(err)
		return http.StatusInternalServerError
	}
	if !allowed {
		return http.StatusForbidden
	}

	return http.StatusOK
}

//
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
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param("ns2"),
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
		Items: []Service{},
		Count: count,
	}
	for _, service := range list {
		r := Service{
			Namespace: service.Namespace,
			Name:      service.Name,
			Object:    service.DecodeObject(),
		}
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific Service on a cluster.
func (h ServiceHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	service := model.Service{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param("ns2"),
			Name:      ctx.Param("service"),
		},
	}
	err := service.Get(h.container.Db)
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
	content := Service{
		Namespace: service.Namespace,
		Name:      service.Name,
		Object:    service.DecodeObject(),
	}

	ctx.JSON(http.StatusOK, content)
}

// Service REST resource
type Service struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Raw k8s object.
	Object *v1.Service `json:"object,omitempty"`
}

//
// Service collection REST resource.
type ServiceList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Service `json:"resources"`
}
