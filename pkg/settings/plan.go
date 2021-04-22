package settings

import (
	"os"
	"strings"

	mapset "github.com/deckarep/golang-set"
)

// Environment variables.
const (
	NsLimit                        = "NAMESPACE_LIMIT"
	PodLimit                       = "POD_LIMIT"
	PvLimit                        = "PV_LIMIT"
	ExcludedResources              = "EXCLUDED_RESOURCES"
	ISResource                     = "imagestreams"
	PVResource                     = "persistentvolumes"
	PVCResource                    = "persistentvolumeclaims"
	EnableIntelligentPVResize      = "ENABLE_INTELLIGENT_PV_RESIZE"
	PvMoveStorageClasses           = "PV_MOVE_STORAGECLASSES"
	PVResizingVolumeUsageThreshold = "PV_RESIZING_USAGE_THRESHOLD"
)

// Included resource defaults
var IncludedInitialResources = mapset.NewSetFromSlice([]interface{}{})
var IncludedStageResources = mapset.NewSetFromSlice([]interface{}{
	"serviceaccount",
	PVResource,
	PVCResource,
	"namespaces",
	ISResource,
	"secrets",
	"configmaps",
	"pods",
})

// Excluded resource defaults
var ExcludedInitialResources = mapset.NewSetFromSlice([]interface{}{
	ISResource,
	PVResource,
	PVCResource,
})
var ExcludedStageResources = mapset.NewSetFromSlice([]interface{}{})

var MoveStorageClasses = mapset.NewSetFromSlice([]interface{}{})

// Plan settings.
//   NsLimit: Maximum number of namespaces on a Plan.
//   PodLimit: Maximum number of Pods across namespaces.
//   PvLimit: Maximum number PVs on a Plan.
//   ExcludedResources: Resources excluded from a Plan.
//   EnableIntelligentPVResize: Enable/Disable PV resizing at plan level
//   PVResizingVolumeUsageThreshold: Usage percentage threshold for pv resizing
type Plan struct {
	NsLimit                        int
	PodLimit                       int
	PvLimit                        int
	EnableIntelligentPVResize      bool
	PVResizingVolumeUsageThreshold int
	ExcludedResources              []string
	MoveStorageClasses             []string
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
	r.EnableIntelligentPVResize = getEnvBool(EnableIntelligentPVResize, false)
	r.PVResizingVolumeUsageThreshold, err = getEnvLimit(PVResizingVolumeUsageThreshold, -1)
	if err != nil {
		return err
	}
	excludedResources := os.Getenv(ExcludedResources)
	if len(excludedResources) > 0 {
		r.ExcludedResources = strings.Split(excludedResources, ",")
	}
	moveStorageClasses := os.Getenv(PvMoveStorageClasses)
	if len(moveStorageClasses) > 0 {
		r.MoveStorageClasses = strings.Split(moveStorageClasses, ",")
	}

	return nil
}
