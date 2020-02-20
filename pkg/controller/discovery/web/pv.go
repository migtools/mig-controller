package web

import (
	"database/sql"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
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
	ClusterScoped
}

//
// Add routes.
func (h PvHandler) AddRoutes(r *gin.Engine) {
	r.GET(PvsRoot, h.List)
	r.GET(PvsRoot+"/", h.List)
	r.GET(PvRoot, h.Get)
}

//
// List all of the PVs on a cluster.
func (h PvHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list, err := h.cluster.PvList(h.container.Db, &h.page)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []v1.PersistentVolume{}
	for _, pv := range list {
		r := v1.PersistentVolume{}
		json.Unmarshal([]byte(pv.Definition), &r)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific PV on a cluster.
func (h PvHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	pv := model.PV{
		Base: model.Base{
			Cluster: h.cluster.PK,
			Name:    ctx.Param("pv"),
		},
	}
	err := pv.Select(h.container.Db)
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
	r := PV{}
	json.Unmarshal([]byte(pv.Definition), &r)
	ctx.JSON(http.StatusOK, r)
}

// PV REST resource
type PV = v1.PersistentVolume
