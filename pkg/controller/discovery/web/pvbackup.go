package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// PodVolumeBackup route root.
const (
	PvBackupParam = "backup"
	PvBackupsRoot = NamespaceRoot + "/podvolumebackups"
	PvBackupRoot  = PvBackupsRoot + "/:" + PvBackupParam
)

// PodVolumeBackup (route) handler.
type PvBackupHandler struct {
	// Base
	ClusterScoped
	// PodVolumeBackup referenced in the request.
	backup model.PodVolumeBackup
}

// Add routes.
func (h PvBackupHandler) AddRoutes(r *gin.Engine) {
	r.GET(PvBackupsRoot, h.List)
	r.GET(PvBackupsRoot+"/", h.List)
	r.GET(PvBackupRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced backup.
func (h *PvBackupHandler) Prepare(ctx *gin.Context) int {
	status := h.ClusterScoped.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(BackupParam)
	if name != "" {
		h.backup = model.PodVolumeBackup{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(PvBackupParam),
			},
		}
		err := h.backup.Get(h.container.Db)
		if err != nil {
			if err != sql.ErrNoRows {
				sink.Trace(err)
				return http.StatusInternalServerError
			} else {
				return http.StatusNotFound
			}
		}
	}

	return http.StatusOK
}

// List all of the backups in the namespace.
func (h PvBackupHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.PodVolumeBackup{}
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
	content := PvBackupList{
		Count: count,
	}
	for _, m := range list {
		r := PvBackup{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific backup.
func (h PvBackupHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.backup.Get(h.container.Db)
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
	r := PvBackup{}
	r.With(&h.backup)
	r.SelfLink = h.Link(&h.cluster, &h.backup)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h PvBackupHandler) Link(c *model.Cluster, m *model.PodVolumeBackup) string {
	return h.BaseHandler.Link(
		PvBackupRoot,
		Params{
			NsParam:       c.Namespace,
			ClusterParam:  c.Name,
			Ns2Param:      m.Namespace,
			PvBackupParam: m.Name,
		})
}

// PodVolumeBackup REST resource.
type PvBackup struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *velero.PodVolumeBackup `json:"object,omitempty"`
}

// Build the resource.
func (r *PvBackup) With(m *model.PodVolumeBackup) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// PodVolumeBackup collection REST resource.
type PvBackupList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []PvBackup `json:"resources"`
}
