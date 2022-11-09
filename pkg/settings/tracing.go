package settings

// Jaeger options
const (
	JaegerEnabled = "JAEGER_ENABLED"
)

// Jaeger Options
//
//	Enabled: whether to emit jaeger spans from controller
type JaegerOpts struct {
	Enabled bool
}

// Load load rsync options
func (r *JaegerOpts) Load() error {
	var err error
	r.Enabled = getEnvBool(JaegerEnabled, false)
	return err
}
