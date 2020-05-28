package settings

// Environment variables.
const (
	RestartRestic = "RESTART_RESTIC"
)

// Migration settings.
//   RestartRestic: bool gating whether restic restart should run
type Migration struct {
	RestartRestic bool
}

// Load settings.
func (r *Migration) Load() error {
	r.RestartRestic = getEnvBool(RestartRestic, false)

	return nil
}
