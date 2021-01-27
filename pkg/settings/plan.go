package settings

import (
	"os"
	"strings"

	mapset "github.com/deckarep/golang-set"
)

// Environment variables.
const (
	NsLimit           = "NAMESPACE_LIMIT"
	PodLimit          = "POD_LIMIT"
	PvLimit           = "PV_LIMIT"
	ExcludedResources = "EXCLUDED_RESOURCES"
	ISResource        = "imagestreams"
	ISTagResource     = "imagestreamtags"
	PVResource        = "persistentvolumes"
	PVCResource       = "persistentvolumeclaims"
)

// Included resource defaults
var IncludedInitialResources = mapset.NewSetFromSlice([]interface{}{})
var IncludedStageResources = mapset.NewSetFromSlice([]interface{}{
	"serviceaccount",
	PVResource,
	PVCResource,
	"namespaces",
	ISResource,
	ISTagResource,
	"secrets",
	"configmaps",
	"pods",
})

// Excluded resource defaults
var ExcludedInitialResources = mapset.NewSetFromSlice([]interface{}{
	ISResource,
	ISTagResource,
	PVResource,
	PVCResource,
})
var ExcludedStageResources = mapset.NewSetFromSlice([]interface{}{})

// Plan settings.
//   NsLimit: Maximum number of namespaces on a Plan.
//   PodLimit: Maximum number of Pods across namespaces.
//   PvLimit: Maximum number PVs on a Plan.
//   ExcludedResources: Resources excluded from a Plan.
type Plan struct {
	NsLimit           int
	PodLimit          int
	PvLimit           int
	ExcludedResources []string
}

// Load settings.
func (r *Plan) Load() error {
	var err error
	r.NsLimit, err = getEnvLimit(NsLimit, 10)
	if err != nil {
		return err
	}
	r.PodLimit, err = getEnvLimit(PodLimit, 100)
	if err != nil {
		return err
	}
	r.PvLimit, err = getEnvLimit(PvLimit, 100)
	if err != nil {
		return err
	}
	excludedResources := os.Getenv(ExcludedResources)
	if len(excludedResources) > 0 {
		r.ExcludedResources = strings.Split(excludedResources, ",")
	}

	return nil
}
