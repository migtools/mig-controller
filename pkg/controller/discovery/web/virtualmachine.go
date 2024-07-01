package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	virtv1 "kubevirt.io/api/core/v1"
)

const (
	VirtualMachineParam = "virtualmachine"
	VirtualMachinesRoot = NamespaceRoot + "/virtualmachines"
	VirtualMachineRoot  = VirtualMachinesRoot + "/:" + VirtualMachineParam
)

// Virtual Machine (route) handler.
type VirtualMachineHandler struct {
	// Base
	ClusterScoped
}

// Add routes.
func (h VirtualMachineHandler) AddRoutes(r *gin.Engine) {
	r.GET(VirtualMachinesRoot, h.List)
	r.GET(VirtualMachinesRoot+"/", h.List)
	r.GET(VirtualMachineRoot, h.Get)
}

// List all of the VirtualMachines on a cluster.
func (h VirtualMachineHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.VirtualMachine{
		Base: model.Base{
			Cluster: h.cluster.PK,
		},
	}
	count, err := collection.Count(db, model.ListOptions{})
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err := collection.List(
		db,
		model.ListOptions{
			Page: &h.page,
		})
	if err != nil {
		sink.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := VirtualMachineList{
		Count: count,
	}
	for _, m := range list {
		r := VirtualMachine{}
		r.With(m)
		r.SelfLink = h.Link(&h.cluster, m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific Virtual Machine on a cluster.
func (h VirtualMachineHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := model.VirtualMachine{
		Base: model.Base{
			Cluster:   h.cluster.PK,
			Namespace: ctx.Param(Ns2Param),
			Name:      ctx.Param(VirtualMachineParam),
		},
	}
	err := m.Get(h.container.Db)
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
	r := VirtualMachine{}
	r.With(&m)
	r.SelfLink = h.Link(&h.cluster, &m)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Build self link.
func (h VirtualMachineHandler) Link(c *model.Cluster, m *model.VirtualMachine) string {
	return h.BaseHandler.Link(
		VirtualMachineRoot,
		Params{
			NsParam:             c.Namespace,
			ClusterParam:        c.Name,
			Ns2Param:            m.Namespace,
			VirtualMachineParam: m.Name,
		})
}

// VirtualMachine REST resource
type VirtualMachine struct {
	// The k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// The k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *virtv1.VirtualMachine `json:"object,omitempty"`
}

// Build the resource.
func (r *VirtualMachine) With(m *model.VirtualMachine) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

// Virtual Machine collection REST resource.
type VirtualMachineList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []VirtualMachine `json:"resources"`
}
