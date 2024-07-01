package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// PodVolumeRestore route root.
const (
	PvRestoreParam = "restore"
	PvRestoresRoot = NamespaceRoot + "/podvolumerestores"
	PvRestoreRoot  = PvRestoresRoot + "/:" + PvRestoreParam
)

// PodVolumeRestore (route) handler.
type PvRestoreHandler struct {
	// Base
	ClusterScoped
	// PodVolumeRestore referenced in the request.
	restore model.PodVolumeRestore
}

// Add routes.
func (h PvRestoreHandler) AddRoutes(r *gin.Engine) {
	r.GET(PvRestoresRoot, h.List)
	r.GET(PvRestoresRoot+"/", h.List)
	r.GET(PvRestoreRoot, h.Get)
}

// Prepare to fulfil the request.
// Fetch the referenced restore.
func (h *PvRestoreHandler) Prepare(ctx *gin.Context) int {
	status := h.ClusterScoped.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	name := ctx.Param(RestoreParam)
	if name != "" {
		h.restore = model.PodVolumeRestore{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: ctx.Param(NsParam),
				Name:      ctx.Param(PvRestoreParam),
			},
		}
		err := h.restore.Get(h.container.Db)
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

// List all of the restores in the namespace.
func (h PvRestoreHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.PodVolumeRestore{}
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
	content := PvRestoreList{
		Count: count,
	}
	for _, m := range list {
		r := PvRestore{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific restore.
func (h PvRestoreHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.restore.Get(h.container.Db)
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
	r := PvRestore{}
	r.With(&h.restore)
	r.SelfLink = h.Link(&h.cluster, &h.restore)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h PvRestoreHandler) Link(c *model.Cluster, m *model.PodVolumeRestore) string {
	return h.BaseHandler.Link(
		PvRestoreRoot,
		Params{
			NsParam:        c.Namespace,
			ClusterParam:   c.Name,
			Ns2Param:       m.Namespace,
			PvRestoreParam: m.Name,
		})
}

// PodVolumeRestore REST resource.
type PvRestore struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *velero.PodVolumeRestore `json:"object,omitempty"`
}

// Build the resource.
func (r *PvRestore) With(m *model.PodVolumeRestore) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// PodVolumeRestore collection REST resource.
type PvRestoreList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []PvRestore `json:"resources"`
}
