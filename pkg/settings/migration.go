package settings

// Environment variables.
const (
	FailureRollback       = "FAILURE_ROLLBACK"
	DisableImageMigration = "DISABLE_IMAGE_MIGRATION"
)

// Migration settings.
//   FailureRollback: Should migration failure result in rollback of resources to source cluster?
//   DisableImageMigration: Disable image migration from CAM
type Migration struct {
	FailureRollback       bool
	DisableImageMigration bool
}

// Load settings.
func (r *Migration) Load() error {
	r.FailureRollback = getEnvBool(FailureRollback, false)
	r.DisableImageMigration = getEnvBool(DisableImageMigration, true)

	return nil
}
