package web

import (
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	Ns2Param       = "ns2"
	NamespacesRoot = ClusterRoot + "/namespaces"
	NamespaceRoot  = NamespacesRoot + "/:" + Ns2Param
)

// Namespaces (route) handler.
type NsHandler struct {
	// Base
	ClusterScoped
}

// Add routes.
func (h NsHandler) AddRoutes(r *gin.Engine) {
	r.GET(NamespacesRoot, h.List)
	r.GET(NamespacesRoot+"/", h.List)
	r.GET(NamespaceRoot, h.Get)
}

// List namespaces on a cluster.
func (h NsHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Namespace{
		Base: model.Base{
			Cluster: h.cluster.PK,
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
			Sort: []int{5},
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := NamespaceList{
		Count: count,
	}
	for _, m := range list {
		podCount, err := model.Pod{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: m.Name,
			},
		}.Count(
			h.container.Db,
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		pvcCount, err := model.PVC{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: m.Name,
			},
		}.Count(
			h.container.Db,
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		srvCount, err := model.Service{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: m.Name,
			},
		}.Count(
			h.container.Db,
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r := Namespace{}
		r.With(m, srvCount, podCount, pvcCount)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific namespace on a cluster.
func (h NsHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}

	ctx.JSON(http.StatusOK, h.cluster.Namespace)
}

// Build self link.
func (h NsHandler) Link(m *model.Namespace) string {
	return h.BaseHandler.Link(
		NamespaceRoot,
		Params{
			Ns2Param: m.Name,
		})
}

// Namespace REST resource
type Namespace struct {
	// Cluster k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// Cluster k8s name.
	Name string `json:"name"`
	// Number of services.
	ServiceCount int64 `json:"serviceCount"`
	// Number of pods.
	PodCount int64 `json:"podCount"`
	// Number of PVCs.
	PvcCount int64 `json:"pvcCount"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *v1.Namespace `json:"object,omitempty"`
}

// Build the resource.
func (r *Namespace) With(m *model.Namespace, serviceCount, podCount, pvcCount int64) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.ServiceCount = serviceCount
	r.PodCount = podCount
	r.PvcCount = pvcCount
}

// NS collection REST resource.
type NamespaceList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Namespace `json:"resources"`
}
