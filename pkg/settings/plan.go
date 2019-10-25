package settings

// Environment variables.
const (
	NsLimit  = "NAMESPACE_LIMIT"
	PodLimit = "POD_LIMIT"
	PvLimit  = "PV_LIMIT"
)

// Plan settings.
//   NsLimit: Maximum number of namespaces on a Plan.
//   PodLimit: Maximum number of Pods across namespaces.
//   PvLimit: Maximum number PVs on a Plan.
type Plan struct {
	NsLimit  int
	PodLimit int
	PvLimit  int
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

	return nil
}
