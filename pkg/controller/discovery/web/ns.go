package web

import (
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	NamespacesRoot = ClusterRoot + "/namespaces"
	NamespaceRoot  = NamespacesRoot + "/:ns2"
)

//
// Namespaces (route) handler.
type NsHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h NsHandler) AddRoutes(r *gin.Engine) {
	r.GET(NamespacesRoot, h.List)
	r.GET(NamespacesRoot+"/", h.List)
	r.GET(NamespaceRoot, h.Get)
}

//
// List namespaces on a cluster.
func (h NsHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list, err := model.Namespace{
		Base: model.Base{
			Cluster: h.cluster.PK,
		},
	}.List(
		h.container.Db,
		model.ListOptions{
			Page: &h.page,
			Sort: []int{5},
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	table := model.Table{Db: h.container.Db}
	content := []Namespace{}
	for _, m := range list {
		podCount, err := table.Count(
			&model.Pod{
				Base: model.Base{
					Cluster:   h.cluster.PK,
					Namespace: m.Name,
				},
			},
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		pvcCount, err := table.Count(
			&model.PVC{
				Base: model.Base{
					Cluster:   h.cluster.PK,
					Namespace: m.Name,
				},
			},
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		SrvCount, err := table.Count(
			&model.Service{
				Base: model.Base{
					Cluster:   h.cluster.PK,
					Namespace: m.Name,
				},
			},
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		content = append(
			content,
			Namespace{
				Name:         m.Name,
				ServiceCount: SrvCount,
				PvcCount:     pvcCount,
				PodCount:     podCount,
			})
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific namespace on a cluster.
func (h NsHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}

	ctx.JSON(http.StatusOK, h.cluster.Namespace)
}

// Namespace REST resource
type Namespace struct {
	// Cluster k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// Cluster k8s name.
	Name string `json:"name"`
	// Raw k8s object.
	Object *v1.Namespace `json:"object,omitempty"`
	// Number of services.
	ServiceCount int `json:"serviceCount"`
	// Number of pods.
	PodCount int `json:"podCount"`
	// Number of PVCs.
	PvcCount int `json:"pvcCount"`
}
