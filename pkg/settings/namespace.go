package settings

import "os"

const (
	Privileged = "PRIVILEGED_NAMESPACE"
	Tenant     = "TENANT_NAMESPACE"
)

// Namespace settings.
type Namespace struct {
	Privileged string
	Tenant     string
}

func (r *Namespace) Load() error {
	if s, found := os.LookupEnv(Tenant); found {
		r.Tenant = s
	}
	if s, found := os.LookupEnv(Privileged); found {
		r.Privileged = s
	}
	return nil
}
