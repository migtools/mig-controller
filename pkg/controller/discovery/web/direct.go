package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	auth "k8s.io/api/authorization/v1"
)

// DirectVolume + DirectImage route root.
const (
	DirectVolumeParam = "directvolume"
	DirectVolumesRoot = Root + "/directvolumes"
	DirectVolumeRoot  = DirectVolumesRoot + "/:" + DirectVolumeParam
	DirectImageParam  = "directimage"
	DirectImagesRoot  = Root + "/directimages"
	DirectImageRoot   = DirectImagesRoot + "/:" + DirectImageParam
)

//
// DirectVolume (route) handler.
type DirectVolumeHandler struct {
	// Base
	BaseHandler
	// DirectVolume referenced in the request.
	directVolume model.DirectVolume
}

// DirectImage (route) handler.
type DirectImageHandler struct {
	// Base
	BaseHandler
	// DirectImage referenced in the request.
	directImage model.DirectImage
}

//
// Add DV routes.
func (h DirectVolumeHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectVolumesRoot, h.List)
	r.GET(DirectVolumesRoot+"/", h.List)
	r.GET(DirectVolumeRoot, h.Get)
}

//
// Add DI routes.
func (h DirectImageHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectImagesRoot, h.List)
	r.GET(DirectImagesRoot+"/", h.List)
	r.GET(DirectImageRoot, h.Get)
}

//
// Prepare to fulfil the request.
// Fetch the referenced dvm.
// Perform SAR authorization.
func (h *DirectVolumeHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectVolumeParam)
	if name != "" {
		h.directVolume = model.DirectVolume{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(DirectVolumeParam),
			},
		}
		err := h.directVolume.Get(h.container.Db)
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

//
// Prepare to fulfil the request.
// Fetch the referenced dim.
// Perform SAR authorization.
func (h *DirectImageHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectImageParam)
	if name != "" {
		h.directImage = model.DirectImage{
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
// The subject is the DirectVolumeMigration.
func (h *DirectVolumeHandler) getSAR() auth.SelfSubjectAccessReview {
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

// Build the appropriate SAR object.
// The subject is the DirectImageMigration.
func (h *DirectImageHandler) getSAR() auth.SelfSubjectAccessReview {
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
// List all of the dvms in the namespace.
func (h DirectVolumeHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.DirectVolume{}
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

//
// List all of the dims in the namespace.
func (h DirectImageHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.DirectImage{}
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
// Get a specific dvm.
func (h DirectVolumeHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.directVolume.Get(h.container.Db)
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
	r := DirectVolume{}
	r.With(&h.directVolume)
	r.SelfLink = h.Link(&h.directVolume)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific dim.
func (h DirectImageHandler) Get(ctx *gin.Context) {
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
func (h DirectVolumeHandler) Link(m *model.DirectVolume) string {
	return h.BaseHandler.Link(
		DirectVolumeRoot,
		Params{
			NsParam:           m.Namespace,
			DirectVolumeParam: m.Name,
		})
}

// // Build self link.
// func (h RestoreHandler) Link(c *model.Cluster, m *model.Restore) string {
// 	return h.BaseHandler.Link(
// 		RestoreRoot,
// 		Params{
// 			NsParam:      c.Namespace,
// 			ClusterParam: c.Name,
// 			Ns2Param:     m.Namespace,
// 			RestoreParam: m.Name,
// 		})
// }

//
// Build self link.
func (h DirectImageHandler) Link(m *model.DirectImage) string {
	return h.BaseHandler.Link(
		DirectImageRoot,
		Params{
			NsParam:          m.Namespace,
			DirectImageParam: m.Name,
		})
}

//
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
func (r *DirectVolume) With(m *model.DirectVolume) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Build the resource.
func (r *DirectImage) With(m *model.DirectImage) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// DirectVolume collection REST resource.
type DirectVolumeList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectVolume `json:"resources"`
}

//
// DirectImage collection REST resource.
type DirectImageList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectImage `json:"resources"`
}
