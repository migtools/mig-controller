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
	DirectVolumeProgressParam  = "directvolumemigrationprogress"
	DirectVolumeProgressesRoot = Root + "/directvolumemigrationprogresses"
	DirectVolumeProgressRoot   = DirectVolumeProgressesRoot + "/:" + DirectVolumeProgressParam
)

// DirectVolumeMigrationProgress (route) handler.
type DirectVolumeMigrationProgressHandler struct {
	// Base
	BaseHandler
	// DirectVolumeMigrationProgress referenced in the request.
	directVolumeProgress model.DirectVolumeMigrationProgress
}

// Add DVMP routes.
func (h DirectVolumeMigrationProgressHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectVolumeProgressesRoot, h.List)
	r.GET(DirectVolumeProgressesRoot+"/", h.List)
	r.GET(DirectVolumeProgressRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced dvmp.
// Perform SAR authorization.
func (h *DirectVolumeMigrationProgressHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectVolumeProgressParam)
	if name != "" {
		h.directVolumeProgress = model.DirectVolumeMigrationProgress{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(DirectVolumeProgressParam),
			},
		}
		err := h.directVolumeProgress.Get(h.container.Db)
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
// The subject is the DirectVolumeMigrationProgress.
func (h *DirectVolumeMigrationProgressHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "DirectVolumeMigrationProgress",
				Namespace: h.directVolumeProgress.Namespace,
				Name:      h.directVolumeProgress.Name,
				Verb:      "get",
			},
		},
	}
}

// List all of the dvmps in the namespace.
func (h DirectVolumeMigrationProgressHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.DirectVolumeMigrationProgress{}
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
	content := DirectVolumeProgressList{
		Count: count,
	}
	for _, m := range list {
		r := DirectVolumeProgress{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific dvmp.
func (h DirectVolumeMigrationProgressHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.directVolumeProgress.Get(h.container.Db)
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
	r := DirectVolumeProgress{}
	r.With(&h.directVolumeProgress)
	r.SelfLink = h.Link(&h.directVolumeProgress)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h DirectVolumeMigrationProgressHandler) Link(m *model.DirectVolumeMigrationProgress) string {
	return h.BaseHandler.Link(
		DirectVolumeProgressRoot,
		Params{
			NsParam:                   m.Namespace,
			DirectVolumeProgressParam: m.Name,
		})
}

// DirectVolumeMigrationProgress REST resource.
type DirectVolumeProgress struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.DirectVolumeMigrationProgress `json:"object,omitempty"`
}

// Build the resource.
func (r *DirectVolumeProgress) With(m *model.DirectVolumeMigrationProgress) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// DirectVolumeProgress collection REST resource.
type DirectVolumeProgressList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectVolumeProgress `json:"resources"`
}
