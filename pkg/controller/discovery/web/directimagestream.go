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
	DirectImageStreamParam = "directimagestreammigration"
	DirectImageStreamsRoot = Root + "/directimagestreammigrations"
	DirectImageStreamRoot  = DirectImageStreamsRoot + "/:" + DirectImageStreamParam
)

// DirectImageStreamMigration (route) handler.
type DirectImageStreamMigrationHandler struct {
	// Base
	BaseHandler
	// DirectImageStreamMigration referenced in the request.
	directImageStream model.DirectImageStreamMigration
}

// Add DISM routes.
func (h DirectImageStreamMigrationHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectImageStreamsRoot, h.List)
	r.GET(DirectImageStreamsRoot+"/", h.List)
	r.GET(DirectImageStreamRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced dism.
// Perform SAR authorization.
func (h *DirectImageStreamMigrationHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectImageStreamParam)
	if name != "" {
		h.directImageStream = model.DirectImageStreamMigration{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(DirectImageStreamParam),
			},
		}
		err := h.directImageStream.Get(h.container.Db)
		if err != nil {
			if err != sql.ErrNoRows {
				Log.Trace(err)
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
// The subject is the DirectImageStreamMigration.
func (h *DirectImageStreamMigrationHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "DirectImageStreamMigration",
				Namespace: h.directImageStream.Namespace,
				Name:      h.directImageStream.Name,
				Verb:      "get",
			},
		},
	}
}

// List all of the disms in the namespace.
func (h DirectImageStreamMigrationHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.DirectImageStreamMigration{}
	count, err := collection.Count(db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err := collection.List(db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := DirectImageStreamList{
		Count: count,
	}
	for _, m := range list {
		r := DirectImageStream{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific dism.
func (h DirectImageStreamMigrationHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.directImageStream.Get(h.container.Db)
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
	r := DirectImageStream{}
	r.With(&h.directImageStream)
	r.SelfLink = h.Link(&h.directImageStream)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h DirectImageStreamMigrationHandler) Link(m *model.DirectImageStreamMigration) string {
	return h.BaseHandler.Link(
		DirectImageStreamRoot,
		Params{
			NsParam:                m.Namespace,
			DirectImageStreamParam: m.Name,
		})
}

// DirectImageStream REST resource.
type DirectImageStream struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.DirectImageStreamMigration `json:"object,omitempty"`
}

// Build the resource.
func (r *DirectImageStream) With(m *model.DirectImageStreamMigration) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// DirectImageStream collection REST resource.
type DirectImageStreamList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectImageStream `json:"resources"`
}
