package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
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
	ClusterScoped
}

//
// Add routes.
func (h ServiceHandler) AddRoutes(r *gin.Engine) {
	r.GET(ServicesRoot, h.List)
	r.GET(ServicesRoot+"/", h.List)
	r.GET(ServiceRoot, h.Get)
}

//
// List all of the Services on a cluster.
func (h ServiceHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list, err := model.Service{
		Base: model.Base{
			Cluster: h.cluster.PK,
		},
	}.List(
		h.container.Db,
		model.ListOptions{
			Page: &h.page,
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Service{}
	for _, service := range list {
		r := Service{
			Namespace: service.Namespace,
			Name:      service.Name,
			Object:    service.DecodeObject(),
		}
		content = append(content, r)
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
