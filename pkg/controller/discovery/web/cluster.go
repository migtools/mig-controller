package web

import (
	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/auth"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"net/http"
)

// Cluster route roots.
const (
	ClustersRoot = Root + "/clusters"
	ClusterRoot  = ClustersRoot + "/:cluster"
)

//
// Cluster (route) handler.
type ClusterHandler struct {
	// Base
	BaseHandler
}

//
// Add routes.
func (h ClusterHandler) AddRoutes(r *gin.Engine) {
	r.GET(ClustersRoot, h.List)
	r.GET(ClustersRoot+"/", h.List)
	r.GET(ClusterRoot, h.Get)
}

//
// Prepare the handler to fulfil the request.
// Perform RBAC authorization.
func (h *ClusterHandler) Prepare(ctx *gin.Context) int {
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
func (h *ClusterHandler) allow(ctx *gin.Context) int {
	allowed, err := h.rbac.Allow(&auth.Review{
		Namespace: h.cluster.Namespace,
		Resource:  "migcluster",
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
// List clusters in the namespace.
func (h ClusterHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Cluster{}
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
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := ClusterList{
		Count: count,
	}
	for _, m := range list {
		d := Cluster{
			Namespace: m.Namespace,
			Name:      m.Name,
			Object:    m.DecodeObject(),
		}
		content.Items = append(content.Items, d)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific cluster.
func (h ClusterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	content := Cluster{
		Namespace: h.cluster.Namespace,
		Name:      h.cluster.Name,
		Object:    h.cluster.DecodeObject(),
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Cluster REST resource.
type Cluster struct {
	// Cluster k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// Cluster k8s name.
	Name string `json:"name"`
	// Raw k8s object.
	Object *migapi.MigCluster `json:"object,omitempty"`
}

//
// Cluster collection REST resource.
type ClusterList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Cluster `json:"resources"`
}
