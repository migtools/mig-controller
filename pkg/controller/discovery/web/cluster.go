package web

import (
	"github.com/gin-gonic/gin"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	auth "k8s.io/api/authorization/v1"
	"net/http"
	"time"
)

// Cluster-scoped route roots.
const (
	ClusterParam = "cluster"
	ClustersRoot = Root + "/clusters"
	ClusterRoot  = ClustersRoot + "/:" + ClusterParam
)

//
// Base handler for cluster scoped routes.
type ClusterScoped struct {
	// Base
	BaseHandler
	// The cluster specified in the request.
	cluster model.Cluster
}

//
// Prepare to fulfil the request.
// Set the `cluster` field and ensure the related DataSource is `ready`.
// Perform SAR authorization.
func (h *ClusterScoped) Prepare(ctx *gin.Context) int {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	h.cluster = model.Cluster{
		CR: model.CR{
			Namespace: ctx.Param(NsParam),
			Name:      ctx.Param(ClusterParam),
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
// Build the appropriate SAR object.
// For clusters without a secret, the cluster itself is the subject.
// For clusters with a secret, the secret is the subject.
// Eventually all clusters should reference secrets and this can
// be simplified.
func (h *ClusterScoped) getSAR() auth.SelfSubjectAccessReview {
	var attributes *auth.ResourceAttributes
	sr := h.cluster.DecodeObject().Spec.ServiceAccountSecretRef
	if sr != nil {
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
func (h *ClusterScoped) dsReady() int {
	wait := time.Second * 30
	poll := time.Microsecond * 100
	for {
		mark := time.Now()
		if ds, found := h.container.GetDs(&h.cluster); found {
			if ds.IsReady() {
				h.cluster.Get(h.container.Db)
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
// Cluster (route) handler.
type ClusterHandler struct {
	// Base
	ClusterScoped
}

//
// Add routes.
func (h ClusterHandler) AddRoutes(r *gin.Engine) {
	r.GET(ClustersRoot, h.List)
	r.GET(ClustersRoot+"/", h.List)
	r.GET(ClusterRoot, h.Get)
}

//
// List clusters in the namespace.
func (h ClusterHandler) List(ctx *gin.Context) {
	status := h.BaseHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.container.Db
	collection := model.Cluster{}
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
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := ClusterList{
		Count: count,
	}
	for _, m := range list {
		r := Cluster{}
		r.With(m)
		r.SelfLink = h.Link(m)
		content.Items = append(content.Items, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific cluster.
func (h ClusterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	r := Cluster{}
	r.With(&h.cluster)
	r.SelfLink = h.Link(&h.cluster)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link.
func (h ClusterHandler) Link(m *model.Cluster) string {
	return h.BaseHandler.Link(
		ClusterRoot,
		Params{
			NsParam:      m.Namespace,
			ClusterParam: m.Name,
		})
}

//
// Cluster REST resource.
type Cluster struct {
	// Cluster k8s namespace.
	Namespace string `json:"namespace,omitempty"`
	// Cluster k8s name.
	Name string `json:"name"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Raw k8s object.
	Object *migapi.MigCluster `json:"object,omitempty"`
}

//
// Get self URI.
func (r *Cluster) With(m *model.Cluster) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m.DecodeObject()
}

//
// Cluster collection REST resource.
type ClusterList struct {
	// Total number in the collection.
	Count int64 `json:"count"`
	// List of resources.
	Items []Cluster `json:"resources"`
}
