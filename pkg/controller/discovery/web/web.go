package web

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/container"
	"github.com/konveyor/mig-controller/pkg/controller/discovery/model"
	"github.com/konveyor/mig-controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/settings"
	auth "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"net/http"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Application settings.
var Settings = &settings.Settings

// Shared logger.
var Log *logging.Logger

// Root - all routes.
const (
	Root = "/namespaces/:ns1"
)

//
// Web server
type WebServer struct {
	// Allowed CORS origins.
	allowedOrigins []*regexp.Regexp
	// Reference to the container.
	Container *container.Container
}

//
// Start the web-server.
// Initializes `gin` with routes and CORS origins.
func (w *WebServer) Start() {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET"},
		AllowHeaders:     []string{"Authorization", "Origin"},
		AllowOriginFunc:  w.allow,
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	w.buildOrigins()
	w.addRoutes(r)
	go r.Run()
}

//
// Build a REGEX for each CORS origin.
func (w *WebServer) buildOrigins() {
	w.allowedOrigins = []*regexp.Regexp{}
	for _, r := range Settings.Discovery.CORS.AllowedOrigins {
		expr, err := regexp.Compile(r)
		if err != nil {
			Log.Error(
				err,
				"origin not valid",
				"expr",
				r)
			continue
		}
		w.allowedOrigins = append(w.allowedOrigins, expr)
		Log.Info(
			"Added allowed origin.",
			"expr",
			r)
	}
}

//
// Add the routes.
func (w *WebServer) addRoutes(r *gin.Engine) {
	handlers := []RequestHandler{
		SchemaHandler{
			router: r,
		},
		RootNsHandler{
			BaseHandler: BaseHandler{
				container: w.Container,
			},
		},
		ClusterHandler{
			ClusterScoped: ClusterScoped{
				BaseHandler: BaseHandler{
					container: w.Container,
				},
			},
		},
		NsHandler{
			ClusterScoped: ClusterScoped{
				BaseHandler: BaseHandler{
					container: w.Container,
				},
			},
		},
		PodHandler{
			ClusterScoped: ClusterScoped{
				BaseHandler: BaseHandler{
					container: w.Container,
				},
			},
		},
		LogHandler{
			ClusterScoped: ClusterScoped{
				BaseHandler: BaseHandler{
					container: w.Container,
				},
			},
		},
		PvHandler{
			ClusterScoped: ClusterScoped{
				BaseHandler: BaseHandler{
					container: w.Container,
				},
			},
		},
		PlanHandler{
			BaseHandler: BaseHandler{
				container: w.Container,
			},
		},
	}
	for _, h := range handlers {
		h.AddRoutes(r)
	}
}

//
// Called by `gin` to perform CORS authorization.
func (w *WebServer) allow(origin string) bool {
	for _, expr := range w.allowedOrigins {
		if expr.MatchString(origin) {
			return true
		}
	}

	Log.Info("Denied.", "origin", origin)

	return false
}

//
// Web request handler.
type RequestHandler interface {
	// Add routes to the `gin` router.
	AddRoutes(*gin.Engine)
	// List resources in a REST collection.
	List(*gin.Context)
	// Get a specific REST resource.
	Get(*gin.Context)
}

//
// Base web request handler.
type BaseHandler struct {
	// Reference to the container used to fulfil requests.
	container *container.Container
	// The bearer token passed in the `Authorization` header.
	token string
	// The `page` parameter passed in the request.
	page model.Page
}

//
// Prepare the handler to fulfil the request.
// Set the `token` and `page` fields using passed parameters.
func (h *BaseHandler) Prepare(ctx *gin.Context) int {
	Log.Reset()
	status := h.setToken(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.setPage(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// Set the `token` field.
func (h *BaseHandler) setToken(ctx *gin.Context) int {
	header := ctx.GetHeader("Authorization")
	fields := strings.Fields(header)
	if len(fields) == 2 && fields[0] == "Bearer" {
		h.token = fields[1]
		return http.StatusOK
	}
	if Settings.Discovery.AuthOptional {
		restCfg, _ := config.GetConfig()
		h.token = restCfg.BearerToken
		return http.StatusOK
	}

	Log.Info("`Authorization: Bearer <token>` header required but not found.")

	return http.StatusBadRequest
}

//
// Set the `token` field.
func (h *BaseHandler) setPage(ctx *gin.Context) int {
	q := ctx.Request.URL.Query()
	page := model.Page{
		Limit:  int(^uint(0) >> 1),
		Offset: 0,
	}
	pLimit := q.Get("limit")
	if len(pLimit) != 0 {
		nLimit, err := strconv.Atoi(pLimit)
		if err != nil || nLimit < 0 {
			return http.StatusBadRequest
		}
		page.Limit = nLimit
	}
	pOffset := q.Get("offset")
	if len(pOffset) != 0 {
		nOffset, err := strconv.Atoi(pOffset)
		if err != nil || nOffset < 0 {
			return http.StatusBadRequest
		}
		page.Offset = nOffset
	}

	h.page = page
	return http.StatusOK
}

//
// Perform SAR.
func (h *BaseHandler) allow(sar auth.SelfSubjectAccessReview) int {
	restCfg, _ := config.GetConfig()
	restCfg.BearerToken = h.token
	codec := serializer.NewCodecFactory(scheme.Scheme)
	gvk, err := apiutil.GVKForObject(&sar, scheme.Scheme)
	if err != nil {
		Log.Trace(err)
		return http.StatusInternalServerError
	}
	restClient, err := apiutil.RESTClientForGVK(gvk, restCfg, codec)
	if err != nil {
		Log.Trace(err)
		return http.StatusInternalServerError
	}
	var status int
	post := restClient.Post()
	post.Resource("selfsubjectaccessreviews")
	post.Body(&sar)
	reply := post.Do()
	reply.StatusCode(&status)
	switch status {
	case http.StatusForbidden, http.StatusUnauthorized:
		return status
	case http.StatusCreated:
		reply.Into(&sar)
		if sar.Status.Allowed {
			return http.StatusOK
		}
	default:
		Log.Info("Unexpected SAR reply", "status", status)
	}

	return http.StatusForbidden
}

//
// Schema (route) handler.
type SchemaHandler struct {
	// The `gin` router.
	router *gin.Engine
}

func (h SchemaHandler) AddRoutes(r *gin.Engine) {
	r.GET("/schema", h.List)
}

//
// List schema.
func (h SchemaHandler) List(ctx *gin.Context) {
	type Schema struct {
		Version string
		Release int
		Paths   []string
	}
	schema := Schema{
		Version: "beta1",
		Release: 2,
		Paths:   []string{},
	}
	for _, rte := range h.router.Routes() {
		schema.Paths = append(schema.Paths, rte.Path)
	}

	ctx.JSON(http.StatusOK, schema)
}

//
// Not supported.
func (h SchemaHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
// Root namespace (route) handler.
type RootNsHandler struct {
	// Base
	BaseHandler
}

func (h RootNsHandler) AddRoutes(r *gin.Engine) {
	r.GET(strings.Split(Root, "/")[1], h.List)
	r.GET(strings.Split(Root, "/")[1]+"/", h.List)
}

//
// List all root level namespaces.
func (h RootNsHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list := []string{}
	set := map[string]bool{}
	// Cluster
	clusters, err := model.ClusterList(h.container.Db, &h.page)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range clusters {
		if _, found := set[m.Namespace]; !found {
			list = append(list, m.Namespace)
			set[m.Namespace] = true
		}

	}
	// Plan
	plans, err := model.PlanList(h.container.Db, &h.page)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range plans {
		if _, found := set[m.Namespace]; !found {
			list = append(list, m.Namespace)
			set[m.Namespace] = true
		}

	}
	sort.Strings(list)
	h.page.Slice(&list)
	ctx.JSON(http.StatusOK, list)
}

//
// Not supported.
func (h RootNsHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}
