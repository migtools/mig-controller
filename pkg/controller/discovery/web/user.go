package web

import (
	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/auth"
	"net/http"
)

// User route roots.
const (
	UserRoot = Root + "/auth"
)

//
// Cluster (route) handler.
type UserHandler struct {
	// Base
	BaseHandler
}

//
// Add routes.
func (h UserHandler) AddRoutes(r *gin.Engine) {
	r.GET(UserRoot, h.Get)
}

//
// List users.
func (h UserHandler) List(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
// Get a cluster user.
func (h UserHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	allowed, err := h.rbac.Allow(
		&auth.Review{
			Group:     migapi.SchemeGroupVersion.Group,
			Namespace: ctx.Param("ns1"),
			Resource:  auth.ALL,
			Verb:      auth.ALL,
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := User{
		Authenticated: h.rbac.Authenticated(),
		HasAdmin:      allowed,
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Cluster authorization
type User struct {
	Authenticated bool `json:"Authenticated"`
	// User has admin.
	HasAdmin bool `json:"hasAdmin"`
}
