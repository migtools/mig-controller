package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	auth "k8s.io/api/authorization/v1"
)

const (
	HookParam = "hook"
	HooksRoot = Root + "/hooks"
	HookRoot  = HooksRoot + "/:" + HookParam
)

// Hook (route) handler.
type HookHandler struct {
	// Base
	BaseHandler
	// Hook referenced in the request.
	hook model.Hook
}

// Add DIM routes.
func (h HookHandler) AddRoutes(r *gin.Engine) {
	r.GET(HooksRoot, h.List)
	r.GET(HooksRoot+"/", h.List)
	r.GET(HookRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced hook.
// Perform SAR authorization.
func (h *HookHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(HookParam)
	if name != "" {
		h.hook = model.Hook{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(HookParam),
			},
		}
		err := h.hook.Get(h.container.Db)
		if err != nil {
			if err != sql.ErrNoRows {
				sink.Trace(err)
				return http.StatusInternalServerError
			} else {
				return http.StatusNotFound
			}
		}
	}
	status = h.allow(h.getSAR())
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

// Build the appropriate SAR object.
// The subject is the Hook.
func (h *HookHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "Hook",
				Namespace: h.hook.Namespace,
				Name:      h.hook.Name,
				Verb:      "get",
			},
		},
	}
}

// List all of the dims in the namespace.
func (h HookHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Hook{}
	count, err := collection.Count(db, model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err := collection.List(db, model.ListOptions{})
	if err != nil {
		sink.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := HookList{
		Count: count,
	}
	for _, m := range list {
		r := Hook{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific dim.
func (h HookHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.hook.Get(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			sink.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		} else {
			ctx.Status(http.StatusNotFound)
			return
		}
	}
	r := Hook{}
	r.With(&h.hook)
	r.SelfLink = h.Link(&h.hook)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h HookHandler) Link(m *model.Hook) string {
	return h.BaseHandler.Link(
		HookRoot,
		Params{
			NsParam:   m.Namespace,
			HookParam: m.Name,
		})
}

// Hook REST resource.
type Hook struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.MigHook `json:"object,omitempty"`
}

// Build the resource.
func (r *Hook) With(m *model.Hook) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// Hook collection REST resource.
type HookList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Hook `json:"resources"`
}
