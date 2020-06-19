package web

import (
	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/auth"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"k8s.io/api/core/v1"
	"net/http"
	"strings"
)

const (
	NamespacesRoot = ClusterRoot + "/namespaces"
	NamespaceRoot  = NamespacesRoot + "/:ns2"
)

//
// Namespaces (route) handler.
type NsHandler struct {
	// Base
	BaseHandler
}

//
// Add routes.
func (h NsHandler) AddRoutes(r *gin.Engine) {
	r.GET(strings.Split(Root, "/")[1], h.ListRoot)
	r.GET(strings.Split(Root, "/")[1]+"/", h.ListRoot)
	r.GET(NamespacesRoot, h.List)
	r.GET(NamespacesRoot+"/", h.List)
	r.GET(NamespaceRoot, h.Get)
}

//
// List root namespaces.
func (h NsHandler) ListRoot(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Namespace{
		Base: model.Base{
			Cluster: h.cluster.PK,
		},
	}
	list, err := collection.List(
		db,
		model.ListOptions{
			Sort: []int{5},
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err = h.hasBasic(list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	count := int64(len(list))
	content := NamespaceList{
		Items: []Namespace{},
		Count: count,
	}
	h.page.Slice(list)
	for _, m := range list {
		content.Items = append(
			content.Items,
			Namespace{
				Name: m.Name,
			})
	}

	ctx.JSON(http.StatusOK, content)
}

//
// List namespaces on a cluster.
func (h NsHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Namespace{
		Base: model.Base{
			Cluster: h.cluster.PK,
		},
	}
	list, err := collection.List(
		db,
		model.ListOptions{
			Sort: []int{5},
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	list, err = h.hasRead(list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	count := int64(len(list))
	content := NamespaceList{
		Items: []Namespace{},
		Count: count,
	}
	h.page.Slice(list)
	for _, m := range list {
		podCount, err := model.Pod{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: m.Name,
			},
		}.Count(
			h.container.Db,
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		pvcCount, err := model.PVC{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: m.Name,
			},
		}.Count(
			h.container.Db,
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		SrvCount, err := model.Service{
			Base: model.Base{
				Cluster:   h.cluster.PK,
				Namespace: m.Name,
			},
		}.Count(
			h.container.Db,
			model.ListOptions{})
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		content.Items = append(
			content.Items,
			Namespace{
				Name:         m.Name,
				ServiceCount: SrvCount,
				PvcCount:     pvcCount,
				PodCount:     podCount,
			})
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific namespace on a cluster.
func (h NsHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
// Token has MIG-admin.
func (h *NsHandler) hasAdmin() (bool, error) {
	return h.rbac.Allow(&auth.Review{
		Group:     migapi.SchemeGroupVersion.Group,
		Namespace: auth.ALL,
		Resource:  auth.ALL,
		Verb:      auth.ALL,
	})
}

//
// Filter the list of namespaces by READ access.
// Returns only those namespaces that the token has READ access.
// Uses `hasAdmin()` as an optimization.  The check is fast and
// efficient and may elliminate a large number of SAR.
func (h *NsHandler) hasRead(models []*model.Namespace) ([]*model.Namespace, error) {
	hasAdmin, err := h.hasAdmin()
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	if hasAdmin {
		return models, nil
	}
	list := []*model.Namespace{}
	for _, ns := range models {
		allowed, err := h.rbac.Allow(&auth.Review{
			Resource:  auth.Namespace,
			Verb:      auth.GET,
			Namespace: ns.Name,
			Name:      ns.Name,
		})
		if err != nil {
			Log.Trace(err)
			return nil, err
		}
		if allowed {
			list = append(list, ns)
		}
	}

	return list, nil
}

//
// Filter the list of namespaces by having MIG basic access.
// Returns only those namespaces that the token has MIG basic access.
// Uses `hasRead()` as an optimization. The check is fast and will
// likely narrow down the list quickly which reduces the number
// of SAR required.
func (h *NsHandler) hasBasic(models []*model.Namespace) ([]*model.Namespace, error) {
	hasAdmin, err := h.hasAdmin()
	if err != nil {
		Log.Trace(err)
		return nil, err
	}
	if hasAdmin {
		return models, nil
	}
	models, err = h.hasRead(models)
	if err != nil {
		return nil, err
	}
	list := []*model.Namespace{}
	for _, ns := range models {
		allowed, err := h.rbac.AllowMatrix(&auth.Matrix{
			Group: migapi.SchemeGroupVersion.Group,
			Resources: []string{
				"migplans",
				"migstorages",
				"migmigrations",
				"migtokens",
			},
			Namespace: ns.Name,
			Verbs: []string{
				auth.ALL,
			}})
		if err != nil {
			Log.Trace(err)
			return nil, err
		}
		if allowed {
			list = append(list, ns)
		}
	}

	return list, err
}

// Namespace REST resource
type Namespace struct {
	// Namespace name.
	Name string `json:"name"`
	// Raw k8s object.
	Object *v1.Namespace `json:"object,omitempty"`
	// Number of services.
	ServiceCount int64 `json:"serviceCount,omitempty"`
	// Number of pods.
	PodCount int64 `json:"podCount,omitempty"`
	// Number of PVCs.
	PvcCount int64 `json:"pvcCount,omitempty"`
}

//
// NS collection REST resource.
type NamespaceList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Namespace `json:"resources"`
}
