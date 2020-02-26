package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	PvcsRoot = NamespaceRoot + "/persistentvolumeclaims"
	PvcRoot  = PvcsRoot + "/:pvc"
)

//
// PVC (route) handler.
type PvcHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h PvcHandler) AddRoutes(r *gin.Engine) {
	r.GET(PvcsRoot, h.List)
	r.GET(PvcsRoot+"/", h.List)
	r.GET(PvcRoot, h.Get)
}

//
// List all of the PVCs on a cluster.
func (h PvcHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list, err := model.PVC{
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
	content := []PVC{}
	for _, pvc := range list {
		r := PVC{
			Namespace: pvc.Namespace,
			Name:      pvc.Name,
			Object:    pvc.DecodeObject(),
		}
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific PVC on a cluster.
func (h PvcHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	pvc := model.PVC{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param("ns2"),
			Name:      ctx.Param("pv"),
		},
	}
	err := pvc.Get(h.container.Db)
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
	content := PVC{
		Namespace: pvc.Namespace,
		Name:      pvc.Name,
		Object:    pvc.DecodeObject(),
	}

	ctx.JSON(http.StatusOK, content)
}

// PVC REST resource
type PVC struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Raw k8s object.
	Object *v1.PersistentVolumeClaim `json:"object,omitempty"`
}
