package web

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"net/http"
)

// Route roots.
const (
	WellKnownRoot = ClusterRoot + WellKnown
	WellKnown     = "/.well-known/oauth-authorization-server"
)

//
// Cluster (route) handler.
type WellKnownHandler struct {
	BaseHandler
}

//
// Add routes.
func (h WellKnownHandler) AddRoutes(r *gin.Engine) {
	r.GET(WellKnownRoot, h.Get)
}

//
// Get the well-know oauth information.
func (h WellKnownHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	cluster := h.cluster.DecodeObject()
	restCfg, err := cluster.BuildRestConfig(h.container.Client)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	url := restCfg.Host + WellKnown
	client, err := h.client(restCfg)
	r, err := client.Get(url)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	if r.StatusCode != http.StatusOK {
		ctx.Status(r.StatusCode)
		return
	}
	defer r.Body.Close()
	content := map[string]interface{}{}
	body, err := ioutil.ReadAll(r.Body)
	err = json.Unmarshal(body, &content)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Build the http client.
func (h *WellKnownHandler) client(restCfg *rest.Config) (*http.Client, error) {
	pool := x509.NewCertPool()
	if restCfg.TLSClientConfig.CAData != nil {
		pool.AppendCertsFromPEM(restCfg.TLSClientConfig.CAData)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: restCfg.TLSClientConfig.Insecure,
			RootCAs:            pool,
		},
	}
	client := &http.Client{Transport: tr}
	return client, nil
}

//
// List.
func (h WellKnownHandler) List(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}
