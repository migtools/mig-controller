package settings

import (
	"os"
	"strings"
)

// Environment variables.
const (
	NsLimit           = "NAMESPACE_LIMIT"
	PodLimit          = "POD_LIMIT"
	PvLimit           = "PV_LIMIT"
	ExcludedResources = "EXCLUDED_RESOURCES"
)

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
