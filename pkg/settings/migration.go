package settings

// Environment variables.
const (
	FailureRollback = "FAILURE_ROLLBACK"
)

// Migration settings.
//   FailureRollback: Should migration failure result in rollback of resources to source cluster?
type Migration struct {
	FailureRollback bool
}

// Load settings.
func (r *Migration) Load() error {
	r.FailureRollback = getEnvBool(FailureRollback, false)

	return nil
}
