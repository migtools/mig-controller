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
	DirectVolumeParam = "directvolumemigration"
	DirectVolumesRoot = Root + "/directvolumemigrations"
	DirectVolumeRoot  = DirectVolumesRoot + "/:" + DirectVolumeParam
)

// DirectVolume (route) handler.
type DirectVolumeMigrationHandler struct {
	// Base
	BaseHandler
	// DirectVolume referenced in the request.
	directVolume model.DirectVolumeMigration
}

// Add DVM routes.
func (h DirectVolumeMigrationHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectVolumesRoot, h.List)
	r.GET(DirectVolumesRoot+"/", h.List)
	r.GET(DirectVolumeRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced dvm.
// Perform SAR authorization.
func (h *DirectVolumeMigrationHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectVolumeParam)
	if name != "" {
		h.directVolume = model.DirectVolumeMigration{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(DirectVolumeParam),
			},
		}
		err := h.directVolume.Get(h.container.Db)
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
// The subject is the DirectVolumeMigration.
func (h *DirectVolumeMigrationHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "DirectVolumeMigration",
				Namespace: h.directVolume.Namespace,
				Name:      h.directVolume.Name,
				Verb:      "get",
			},
		},
	}
}

// List all of the dvms in the namespace.
func (h DirectVolumeMigrationHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.DirectVolumeMigration{}
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
	content := DirectVolumeList{
		Count: count,
	}
	for _, m := range list {
		r := DirectVolume{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific dvm.
func (h DirectVolumeMigrationHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.directVolume.Get(h.container.Db)
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
	r := DirectVolume{}
	r.With(&h.directVolume)
	r.SelfLink = h.Link(&h.directVolume)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h DirectVolumeMigrationHandler) Link(m *model.DirectVolumeMigration) string {
	return h.BaseHandler.Link(
		DirectVolumeRoot,
		Params{
			NsParam:           m.Namespace,
			DirectVolumeParam: m.Name,
		})
}

// DirectVolume REST resource.
type DirectVolume struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.DirectVolumeMigration `json:"object,omitempty"`
}

// Build the resource.
func (r *DirectVolume) With(m *model.DirectVolumeMigration) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// DirectVolume collection REST resource.
type DirectVolumeList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectVolume `json:"resources"`
}
