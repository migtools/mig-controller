package web

import (
	"database/sql"
	"encoding/json"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"github.com/gin-gonic/gin"
	"k8s.io/api/core/v1"
	"net/http"
)

const (
	PvsRoot = ClusterRoot + "/persistentvolumes"
	PvRoot  = PvsRoot + "/:pv"
)

//
// PV (route) handler.
type PvHandler struct {
	// Base
	ClusterHandler
}

//
// Add routes.
func (h *PvHandler) AddRoutes(r *gin.Engine) {
	r.GET(PvsRoot, h.List)
	r.GET(PvsRoot+"/", h.List)
	r.GET(PvRoot, h.Get)
}

//
// Get a specific PV on a cluster.
func (h *PvHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		h.ctx.Status(status)
		return
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: h.cluster.PK,
			Name:    h.ctx.Param("pv"),
		},
	}
	err := pv.Select(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			h.ctx.Status(http.StatusInternalServerError)
			return
		} else {
			h.ctx.Status(http.StatusNotFound)
			return
		}
	}
	r := v1.PersistentVolume{}
	json.Unmarshal([]byte(pv.Definition), &r)
	h.ctx.JSON(http.StatusOK, r)
}

//
// List all of the PVs on a cluster.
func (h *PvHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		h.ctx.Status(status)
		return
	}
	list, err := h.cluster.PvList(h.container.Db, &h.page)
	if err != nil {
		Log.Trace(err)
		h.ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []v1.PersistentVolume{}
	for _, pv := range list {
		r := v1.PersistentVolume{}
		json.Unmarshal([]byte(pv.Definition), &r)
		content = append(content, r)
	}

	h.ctx.JSON(http.StatusOK, content)
}
