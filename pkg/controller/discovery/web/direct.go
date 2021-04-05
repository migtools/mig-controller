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
	DirectVolumeParam = "directvolumemigration"
	DirectVolumesRoot = Root + "/directvolumemigrations"
	DirectVolumeRoot  = DirectVolumesRoot + "/:" + DirectVolumeParam

	DirectImageParam = "directimagemigration"
	DirectImagesRoot = Root + "/directimagemigrations"
	DirectImageRoot  = DirectImagesRoot + "/:" + DirectImageParam

	DirectImageStreamParam = "directimagestreammigration"
	DirectImageStreamsRoot = Root + "/directimagestreammigrations"
	DirectImageStreamRoot  = DirectImageStreamsRoot + "/:" + DirectImageStreamParam

	DirectVolumeProgressParam  = "directvolumemigrationprogress"
	DirectVolumeProgressesRoot = Root + "/directvolumemigrationprogresses"
	DirectVolumeProgressRoot   = DirectVolumeProgressesRoot + "/:" + DirectVolumeProgressParam
)

//
// DirectVolume (route) handler.
type DirectVolumeMigrationHandler struct {
	// Base
	BaseHandler
	// DirectVolume referenced in the request.
	directVolume model.DirectVolumeMigration
}

// DirectImage (route) handler.
type DirectImageMigrationHandler struct {
	// Base
	BaseHandler
	// DirectImage referenced in the request.
	directImage model.DirectImageMigration
}

// DirectImageStreamMigration (route) handler.
type DirectImageStreamMigrationHandler struct {
	// Base
	BaseHandler
	// DirectImageStreamMigration referenced in the request.
	directImageStream model.DirectImageStreamMigration
}

// DirectVolumeMigrationProgress (route) handler.
type DirectVolumeMigrationProgressHandler struct {
	// Base
	BaseHandler
	// DirectVolumeMigrationProgress referenced in the request.
	directVolumeProgress model.DirectVolumeMigrationProgress
}

//
// Add DVM routes.
func (h DirectVolumeMigrationHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectVolumesRoot, h.List)
	r.GET(DirectVolumesRoot+"/", h.List)
	r.GET(DirectVolumeRoot, h.Get)
}

//
// Add DIM routes.
func (h DirectImageMigrationHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectImagesRoot, h.List)
	r.GET(DirectImagesRoot+"/", h.List)
	r.GET(DirectImageRoot, h.Get)
}

//
// Add DISM routes.
func (h DirectImageStreamMigrationHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectImageStreamsRoot, h.List)
	r.GET(DirectImageStreamsRoot+"/", h.List)
	r.GET(DirectImageStreamRoot, h.Get)
}

//
// Add DVMP routes.
func (h DirectVolumeMigrationProgressHandler) AddRoutes(r *gin.Engine) {
	r.GET(DirectVolumeProgressesRoot, h.List)
	r.GET(DirectVolumeProgressesRoot+"/", h.List)
	r.GET(DirectVolumeProgressRoot, h.Get)
}

//
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

//
// Prepare to fulfil the request.
// Fetch the referenced dism.
// Perform SAR authorization.
func (h *DirectImageStreamMigrationHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectVolumeParam)
	if name != "" {
		h.directImageStream = model.DirectImageStreamMigration{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(DirectVolumeParam),
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

//
// Prepare to fulfil the request.
// Fetch the referenced dvmp.
// Perform SAR authorization.
func (h *DirectVolumeMigrationProgressHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(DirectVolumeParam)
	if name != "" {
		h.directVolumeProgress = model.DirectVolumeMigrationProgress{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(DirectVolumeParam),
			},
		}
		err := h.directVolumeProgress.Get(h.container.Db)
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

//
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

//
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

//
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

//
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
			Log.Trace(err)
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

//
// Build self link.
func (h DirectVolumeMigrationHandler) Link(m *model.DirectVolumeMigration) string {
	return h.BaseHandler.Link(
		DirectVolumeRoot,
		Params{
			NsParam:           m.Namespace,
			DirectVolumeParam: m.Name,
		})
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
// Build self link.
func (h DirectImageStreamMigrationHandler) Link(m *model.DirectImageStreamMigration) string {
	return h.BaseHandler.Link(
		DirectImageStreamRoot,
		Params{
			NsParam:                m.Namespace,
			DirectImageStreamParam: m.Name,
		})
}

//
// Build self link.
func (h DirectVolumeMigrationProgressHandler) Link(m *model.DirectVolumeMigrationProgress) string {
	return h.BaseHandler.Link(
		DirectVolumeProgressRoot,
		Params{
			NsParam:                   m.Namespace,
			DirectVolumeProgressParam: m.Name,
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

//
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

//
// Build the resource.
func (r *DirectVolume) With(m *model.DirectVolumeMigration) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Build the resource.
func (r *DirectImage) With(m *model.DirectImageMigration) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Build the resource.
func (r *DirectImageStream) With(m *model.DirectImageStreamMigration) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Build the resource.
func (r *DirectVolumeProgress) With(m *model.DirectVolumeMigrationProgress) {
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

//
// DirectImageStream collection REST resource.
type DirectImageStreamList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectImageStream `json:"resources"`
}

//
// DirectVolumeProgress collection REST resource.
type DirectVolumeProgressList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []DirectVolumeProgress `json:"resources"`
}
