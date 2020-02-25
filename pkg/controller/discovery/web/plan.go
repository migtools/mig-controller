package web

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	auth "k8s.io/api/authorization/v1"
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
// Perform SAR authorization.
func (h *PlanHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	h.plan = model.Plan{
		CR: model.CR{
			Namespace: ctx.Param("ns1"),
			Name:      ctx.Param("plan"),
		},
	}
	err := h.plan.Get(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			return http.StatusInternalServerError
		} else {
			return http.StatusNotFound
		}
	}
	status = h.allow(h.getSAR())
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

// Build the appropriate SAR object.
// The subject is the MigPlan.
func (h *PlanHandler) getSAR() auth.SelfSubjectAccessReview {
	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &auth.ResourceAttributes{
				Group:     "apps",
				Resource:  "MigPlan",
				Namespace: h.plan.Namespace,
				Name:      h.plan.Name,
				Verb:      "get",
			},
		},
	}
}

//
// List all of the plans in the namespace.
func (h PlanHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list, err := model.Plan{}.List(h.container.Db, model.ListOptions{})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Plan{}
	for _, m := range list {
		d := Plan{
			Namespace: m.Namespace,
			Name:      m.Name,
			Object:    m.DecodeObject(),
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
	err := h.plan.Get(h.container.Db)
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
		Namespace: h.plan.Namespace,
		Name:      h.plan.Name,
		Object:    h.plan.DecodeObject(),
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
	// Raw k8s object.
	Object *migapi.MigPlan `json:"object"`
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
	object := h.plan.DecodeObject()
	if err != nil {
		Log.Trace(err)
		return err
	}
	p.Destination, err = p.buildPods(h, object.Spec.SrcMigClusterRef)
	if err != nil {
		Log.Trace(err)
		return err
	}
	p.Source, err = p.buildPods(h, object.Spec.DestMigClusterRef)
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
	list, err := model.Pod{
		Base: model.Base{
			Namespace: h.plan.Namespace,
		},
	}.List(
		h.container.Db,
		model.ListOptions{
			Labels: model.Labels{
				"control-plane": "controller-manager",
				"app":           "migration",
			},
		})
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
		CR: model.CR{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
	}
	err := cluster.Get(h.container.Db)
	if err != nil {
		if err != sql.ErrNoRows {
			Log.Trace(err)
			return nil, err
		} else {
			return nil, nil
		}
	}
	labels := []model.Labels{
		{"component": "velero"},
		{"name": "restic"},
	}
	for _, lb := range labels {
		podModels, err := model.Pod{
			Base: model.Base{
				Cluster:   cluster.PK,
				Namespace: cluster.Namespace,
			},
		}.List(
			h.container.Db,
			model.ListOptions{
				Labels: lb,
			})
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
