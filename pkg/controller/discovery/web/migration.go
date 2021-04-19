package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	auth "k8s.io/api/authorization/v1"
)

// Migration route root.
const (
	MigrationParam = "migration"
	MigrationsRoot = Root + "/migrations"
	MigrationRoot  = MigrationsRoot + "/:" + MigrationParam
)

//
// Migration (route) handler.
type MigrationHandler struct {
	// Base
	BaseHandler
	// Migration referenced in the request.
	migration model.Migration
}

//
// Add routes.
func (h MigrationHandler) AddRoutes(r *gin.Engine) {
	r.GET(MigrationsRoot, h.List)
	r.GET(MigrationsRoot+"/", h.List)
	r.GET(MigrationRoot, h.Get)
}

//
// Prepare to fulfil the request.
// Fetch the referenced migration.
// Perform SAR authorization.
func (h *MigrationHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(MigrationParam)
	if name != "" {
		h.migration = model.Migration{
			CR: model.CR{
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(MigrationParam),
			},
		}
		err := h.migration.Get(h.container.Db)
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
// The subject is the MigMigration.
func (h *MigrationHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "MigMigration",
				Namespace: h.migration.Namespace,
				Name:      h.migration.Name,
				Verb:      "get",
			},
		},
	}
}

//
// List all of the migrations in the namespace.
func (h MigrationHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Migration{}
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
	content := MigrationList{
		Count: count,
	}
	for _, m := range list {
		r := Migration{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific migration.
func (h MigrationHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.migration.Get(h.container.Db)
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
	r := Migration{}
	r.With(&h.migration)
	r.SelfLink = h.Link(&h.migration)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h MigrationHandler) Link(m *model.Migration) string {
	return h.BaseHandler.Link(
		MigrationRoot,
		Params{
			NsParam:        m.Namespace,
			MigrationParam: m.Name,
		})
}

//
// Migration REST resource.
type Migration struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.MigMigration `json:"object,omitempty"`
}

//
// Build the resource.
func (r *Migration) With(m *model.Migration) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Migration collection REST resource.
type MigrationList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Migration `json:"resources"`
}
