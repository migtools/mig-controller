package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"net/http"
)

// Backup route root.
const (
	BackupParam = "backup"
	BackupsRoot = NamespaceRoot + "/backups"
	BackupRoot  = BackupsRoot + "/:" + BackupParam
)

// Backup (route) handler.
type BackupHandler struct {
	// Base
	ClusterScoped
	// Backup referenced in the request.
	backup model.Backup
}

// Add routes.
func (h BackupHandler) AddRoutes(r *gin.Engine) {
	r.GET(BackupsRoot, h.List)
	r.GET(BackupsRoot+"/", h.List)
	r.GET(BackupRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced backup.
func (h *BackupHandler) Prepare(ctx *gin.Context) int {
	status := h.ClusterScoped.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(BackupParam)
	if name != "" {
		h.backup = model.Backup{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: ctx.Param(Ns2Param),
				Name:      ctx.Param(BackupParam),
			},
		}
		err := h.backup.Get(h.container.Db)
		if err != nil {
			if err != sql.ErrNoRows {
				Log.Trace(err)
				return http.StatusInternalServerError
			} else {
				return http.StatusNotFound
			}
		}
	}

	return http.StatusOK
}

// List all of the backups in the namespace.
func (h BackupHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Backup{}
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
	content := BackupList{
		Count: count,
	}
	for _, m := range list {
		r := Backup{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific backup.
func (h BackupHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.backup.Get(h.container.Db)
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
	r := Backup{}
	r.With(&h.backup)
	r.SelfLink = h.Link(&h.cluster, &h.backup)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h BackupHandler) Link(c *model.Cluster, m *model.Backup) string {
	return h.BaseHandler.Link(
		BackupRoot,
		Params{
			NsParam:      c.Namespace,
			ClusterParam: c.Name,
			Ns2Param:     m.Namespace,
			BackupParam:  m.Name,
		})
}

// Backup REST resource.
type Backup struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *velero.Backup `json:"object,omitempty"`
}

// Build the resource.
func (r *Backup) With(m *model.Backup) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// Backup collection REST resource.
type BackupList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Backup `json:"resources"`
}
