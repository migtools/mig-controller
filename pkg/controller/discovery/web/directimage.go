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
	DirectImageParam = "directimagemigration"
	DirectImagesRoot = Root + "/directimagemigrations"
	DirectImageRoot  = DirectImagesRoot + "/:" + DirectImageParam
)

// DirectImage (route) handler.
type DirectImageMigrationHandler struct {
	// Base
	BaseHandler
	// DirectImage referenced in the request.
	directImage model.DirectImageMigration
}

//
// Add DIM routes.
func (h DirectImageMigrationHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectImagesRoot, h.List)
	r.GET(DirectImagesRoot+"/", h.List)
	r.GET(DirectImageRoot, h.Get)
}

//
// Prepare to fulfil the request.
// Fetch the referenced dim.
// Perform SAR authorization.
func (h *DirectImageMigrationHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectImageParam)
	if name != "" {
		h.directImage = model.DirectImageMigration{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(DirectImageParam),
			},
		}
		err := h.directImage.Get(h.container.Db)
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
// The subject is the DirectImageMigration.
func (h *DirectImageMigrationHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "DirectImageMigration",
				Namespace: h.directImage.Namespace,
				Name:      h.directImage.Name,
				Verb:      "get",
			},
		},
	}
}

//
// List all of the dims in the namespace.
func (h DirectImageMigrationHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.DirectImageMigration{}
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
	content := DirectImageList{
		Count: count,
	}
	for _, m := range list {
		r := DirectImage{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific dim.
func (h DirectImageMigrationHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.directImage.Get(h.container.Db)
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
	r := DirectImage{}
	r.With(&h.directImage)
	r.SelfLink = h.Link(&h.directImage)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h DirectImageMigrationHandler) Link(m *model.DirectImageMigration) string {
	return h.BaseHandler.Link(
		DirectImageRoot,
		Params{
			NsParam:          m.Namespace,
			DirectImageParam: m.Name,
		})
}

//
// DirectImage REST resource.
type DirectImage struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.DirectImageMigration `json:"object,omitempty"`
}

//
// Build the resource.
func (r *DirectImage) With(m *model.DirectImageMigration) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// DirectImage collection REST resource.
type DirectImageList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectImage `json:"resources"`
}
