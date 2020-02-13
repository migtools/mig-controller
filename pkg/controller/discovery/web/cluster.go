package web

import (
	"github.com/fusor/mig-controller/pkg/controller/discovery/auth"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"github.com/gin-gonic/gin"
	"k8s.io/api/core/v1"
	"net/http"
)

// Cluster-scoped route roots.
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
	allowed, err := h.rbac.Allow(&auth.Request{
		Namespace: h.cluster.Namespace,
		Resources: []string{
			auth.Namespace,
		},
		Verbs: []string{
			auth.GET,
		},
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
	list, err := model.ClusterList(h.container.Db, &h.page)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Cluster{}
	for _, m := range list {
		r := Cluster{
			Namespace: m.Namespace,
			Name:      m.Name,
			Secret:    m.DecodeSecret(),
		}
		content = append(content, r)
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
		Secret:    h.cluster.DecodeSecret(),
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Cluster REST resource.
type Cluster struct {
	// Cluster k8s namespace.
	Namespace string `json:"namespace"`
	// Cluster k8s name.
	Name string `json:"name"`
	// Cluster json-encode secret ref.
	Secret *v1.ObjectReference `json:"secret"`
}
