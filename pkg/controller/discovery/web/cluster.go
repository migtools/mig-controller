package web

import (
	"github.com/fusor/mig-controller/pkg/controller/discovery/model"
	"github.com/gin-gonic/gin"
	auth "k8s.io/api/authorization/v1"
	"k8s.io/api/core/v1"
	"net/http"
	"time"
)

// Cluster-based route roots.
const (
	ClustersRoot = Root + "/clusters"
	ClusterRoot  = ClustersRoot + "/:cluster"
)

//
// Cluster (route) Handler
type ClusterHandler struct {
	// Base
	BaseHandler
	// The cluster specified in the request.
	cluster model.Cluster
}

//
// Add routes.
func (h *ClusterHandler) AddRoutes(r *gin.Engine) {
	r.GET(ClustersRoot, h.List)
	r.GET(ClustersRoot+"/", h.List)
	r.GET(ClusterRoot, h.Get)
}

//
// Prepare to fulfil the request.
// Set the `cluster` field and ensure the related DataSource is `ready`.
// Perform SAR authorization.
func (h *ClusterHandler) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	h.cluster = model.Cluster{
		Base: model.Base{
			Namespace: h.ctx.Param("namespace"),
			Name:      h.ctx.Param("cluster"),
		},
	}
	status = h.dsReady()
	if status != http.StatusOK {
		return status
	}
	status = h.allow(h.getSAR())
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// List clusters in the namespace.
func (h *ClusterHandler) List(ctx *gin.Context) {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		h.ctx.Status(status)
		return
	}
	list, err := model.ClusterList(h.container.Db, &h.page)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []Cluster{}
	for _, m := range list {
		r := Cluster{
			Namespace: m.Namespace,
			Name:      m.Name,
			Secret:    m.DecodeSecret(),
		}
		content = append(content, r)
	}

	h.ctx.JSON(http.StatusOK, content)
}

//
// Get a specific cluster.
func (h *ClusterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		h.ctx.Status(status)
		return
	}
	content := Cluster{
		Namespace: h.cluster.Namespace,
		Name:      h.cluster.Name,
		Secret:    h.cluster.DecodeSecret(),
	}

	h.ctx.JSON(http.StatusOK, content)
}

//
// Build the appropriate SAR object.
// For clusters without a secret, the cluster itself is the subject.
// For clusters with a secret, the secret is the subject.
// Eventually all clusters should reference secrets and this can
// be simplified.
func (h *ClusterHandler) getSAR() auth.SelfSubjectAccessReview {
	var attributes *auth.ResourceAttributes
	if h.cluster.Secret != "" {
		sr := h.cluster.DecodeSecret()
		attributes = &auth.ResourceAttributes{
			Group:     "apps",
			Resource:  "secret",
			Namespace: sr.Namespace,
			Name:      sr.Name,
			Verb:      "get",
		}
	} else {
		attributes = &auth.ResourceAttributes{
			Group:     "apps",
			Resource:  "MigCluster",
			Namespace: h.cluster.Namespace,
			Name:      h.cluster.Name,
			Verb:      "get",
		}
	}

	return auth.SelfSubjectAccessReview{
		Spec: auth.SelfSubjectAccessReviewSpec{
			ResourceAttributes: attributes,
		},
	}
}

//
// Ensure the DataSource associated with the cluster is `ready`.
func (h *ClusterHandler) dsReady() int {
	wait := time.Second * 30
	poll := time.Microsecond * 100
	for {
		mark := time.Now()
		if ds, found := h.container.GetDs(&h.cluster); found {
			if ds.IsReady() {
				h.cluster.Select(h.container.Db)
				return http.StatusOK
			}
		}
		hasCluster, err := h.container.HasCluster(&h.cluster)
		if err != nil {
			return http.StatusInternalServerError
		}
		if !hasCluster {
			return http.StatusNotFound
		}
		if wait > 0 {
			time.Sleep(poll)
			wait -= time.Since(mark)
		} else {
			break
		}
	}

	return http.StatusPartialContent
}

//
// Cluster REST resource.
type Cluster struct {
	Namespace string              `json:"namespace"`
	Name      string              `json:"name"`
	Secret    *v1.ObjectReference `json:"secret"`
}
