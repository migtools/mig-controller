package web

import (
	"github.com/fusor/mig-controller/pkg/controller/discovery/auth"
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
	BaseHandler
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
		h.ctx.Status(status)
		return
	}
	list, err := h.cluster.NsList(h.container.Db, &h.page)
	if err != nil {
		Log.Trace(err)
		h.ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Namespace{}
	subject := &auth.Request{
		Resource: auth.ANY,
		Verbs:    auth.EDIT,
		Local:    true,
	}
	for _, ns := range list {
		subject.Namespace = ns.Name
		allow, err := h.rbac.Allow(subject)
		if err != nil {
			Log.Trace(err)
			h.ctx.Status(http.StatusInternalServerError)
			return
		}
		if allow {
			content = append(content, ns.Name)
		}
	}

	h.ctx.JSON(http.StatusOK, content)
}

//
// Get a specific namespace on a cluster.
func (h NsHandler) Get(ctx *gin.Context) {
	h.ctx.Status(http.StatusMethodNotAllowed)
}

//
// Namespace REST resource
type Namespace = string
