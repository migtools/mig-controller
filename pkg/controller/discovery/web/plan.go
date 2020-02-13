package web

import (
	"database/sql"
	"github.com/fusor/mig-controller/pkg/controller/discovery/auth"
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"github.com/gin-gonic/gin"
	"k8s.io/api/core/v1"
	"net/http"
)

// Plan route root.
const (
	PlansRoot = Root + "/plans"
	PlanRoot  = PlansRoot + "/:plan"
)

//
// Plan (route) handler.
type PlanHandler struct {
	// Base
	BaseHandler
	// Plan referenced in the request.
	plan model.Plan
}

//
// Add routes.
func (h PlanHandler) AddRoutes(r *gin.Engine) {
	r.GET(PlansRoot, h.List)
	r.GET(PlansRoot+"/", h.List)
	r.GET(PlanRoot, h.Get)
	r.GET(PlanRoot+"/pods", h.Pods)
}

//
// Prepare to fulfil the request.
// Fetch the referenced plan.
func (h *PlanHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.allow(ctx)
	if status != http.StatusOK {
		return status
	}
	h.plan = model.Plan{
		Base: model.Base{
			Namespace: ctx.Param("namespace"),
			Name:      ctx.Param("plan"),
		},
	}

	return http.StatusOK
}

//
// RBAC authorization.
func (h *PlanHandler) allow(ctx *gin.Context) int {
	allowed, err := h.rbac.Allow(&auth.Request{
		Namespace: h.cluster.Namespace,
		Resources: []string{
			auth.Namespace,
		},
		Verbs: []string{
			auth.GET,
		},
	})
	if err != nil {
		Log.Trace(err)
		return http.StatusInternalServerError
	}
	if !allowed {
		return http.StatusForbidden
	}

	return http.StatusOK
}

//
// List all of the plans in the namespace.
func (h PlanHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list, err := model.PlanList(h.container.Db, &h.page)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Plan{}
	for _, m := range list {
		d := Plan{
			Namespace:   m.Namespace,
			Name:        m.Name,
			Source:      m.DecodeSource(),
			Destination: m.DecodeDestination(),
		}
		content = append(content, d)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific plan.
func (h PlanHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	err := h.plan.Select(h.container.Db)
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
	content := Plan{
		Namespace:   h.plan.Namespace,
		Name:        h.plan.Name,
		Source:      h.plan.DecodeSource(),
		Destination: h.plan.DecodeDestination(),
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get the `PlanPods` for the plan.
func (h PlanHandler) Pods(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	content := PlanPods{}
	content.With(ctx, &h)

	ctx.JSON(http.StatusOK, content)
}

//
// Plan REST resource.
type Plan struct {
	// The k8s namespace.
	Namespace string `json:"namespace"`
	// The k8s name.
	Name string `json:"name"`
	// A reference to the source cluster.
	Source *v1.ObjectReference `json:"source"`
	// A reference to the destination cluster.
	Destination *v1.ObjectReference `json:"destination"`
}

//
// PlanPods REST resource.
type PlanPods struct {
	// The k8s namespace.
	Namespace string `json:"namespace"`
	// The k8s name.
	Name string `json:"name"`
	// List of controller pods.
	Controller []Pod `json:"controller"`
	// List of relevant pods on source cluster.
	Source []Pod `json:"source"`
	// List of relevant pods on the destination cluster.
	Destination []Pod `json:"destination"`
}

//
// Update the model `with` a PlanHandler
// Fetch and build the pod lists.
func (p *PlanPods) With(ctx *gin.Context, h *PlanHandler) error {
	var err error
	p.Namespace = h.plan.Namespace
	p.Name = h.plan.Name
	p.Controller = []Pod{}
	p.Source = []Pod{}
	p.Destination = []Pod{}
	p.Controller, err = p.buildController(ctx, h)
	if err != nil {
		Log.Trace(err)
		return err
	}
	p.Destination, err = p.buildPods(h, h.plan.DecodeDestination())
	if err != nil {
		Log.Trace(err)
		return err
	}
	p.Source, err = p.buildPods(h, h.plan.DecodeSource())
	if err != nil {
		Log.Trace(err)
		return err
	}

	return nil
}

//
// Build the controller pods list.
// Finds pods by label.
func (p *PlanPods) buildController(ctx *gin.Context, h *PlanHandler) ([]Pod, error) {
	pods := []Pod{}
	list, err := model.PodListByLabel(
		h.container.Db,
		model.LabelFilter{
			model.AsLabel("control-plane", "controller-manager"),
			model.AsLabel("app", "migration"),
		},
		nil)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return nil, err
	}
	if len(list) == 0 {
		return pods, nil
	}
	m := list[0]
	cluster, err := m.GetCluster(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			return nil, err
		} else {
			return pods, nil
		}
	}
	pod := Pod{}
	filter := func(c *v1.Container) bool { return c.Name == "cam" }
	pod.With(m, cluster, filter)
	pods = append(pods, pod)

	return pods, nil
}

//
// Build list of relevant pods on a cluster.
// Finds pods by label. Currently includes velero and restic pods.
func (p *PlanPods) buildPods(h *PlanHandler, ref *v1.ObjectReference) ([]Pod, error) {
	pods := []Pod{}
	cluster := model.Cluster{
		Base: model.Base{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
	}
	err := cluster.Select(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			return nil, err
		} else {
			return nil, nil
		}
	}
	labels := []model.Label{
		model.AsLabel("component", "velero"),
		model.AsLabel("name", "restic"),
	}
	for _, lb := range labels {
		filter := model.LabelFilter{lb}
		podModels, err := cluster.PodListByLabel(h.container.Db, filter, nil)
		if err != nil {
			Log.Trace(err)
			return nil, err
		}
		for _, model := range podModels {
			pod := Pod{}
			pod.With(model, &cluster)
			pods = append(pods, pod)
		}
	}

	return pods, nil
}
