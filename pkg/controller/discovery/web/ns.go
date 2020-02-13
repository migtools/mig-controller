package web

import (
	"github.com/gin-gonic/gin"
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
	list, err := h.cluster.NsList(h.container.Db, &h.page)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Namespace{}
	for _, m := range list {
		content = append(content, m.Name)
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
type Namespace = string
