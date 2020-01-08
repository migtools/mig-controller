package settings

import (
	"os"
	"strings"
)

// Environment variables.
const (
	AllowedOrigins = "CORS_ALLOWED_ORIGINS"
	WorkingDir     = "WORKING_DIR"
	AuthOptinal    = "AUTH_OPTIONAL"
)

// CORS
//   AllowedOrigins: Allowed origins.
type CORS struct {
	AllowedOrigins []string
}

// Plan settings.
//   Origins: Permitted CORS allowed origins.
//   AuthOptional: Authorization header is optional.
type Discovery struct {
	CORS         CORS
	WorkingDir   string
	AuthOptional bool
}

// Load settings.
func (r *Discovery) Load() error {
	r.CORS = CORS{
		AllowedOrigins: []string{},
	}
	// AllowedOrigins
	if s, found := os.LookupEnv(AllowedOrigins); found {
		parts := strings.Fields(s)
		for _, s := range parts {
			if len(s) > 0 {
				r.CORS.AllowedOrigins = append(r.CORS.AllowedOrigins, s)
			}
		}
	}
	// WorkingDir
	if s, found := os.LookupEnv(WorkingDir); found {
		r.WorkingDir = s
	} else {
		r.WorkingDir = os.TempDir()
	}
	// Auth
	r.AuthOptional = getEnvBool(AuthOptinal, false)

	return nil
}
