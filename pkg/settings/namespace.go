package settings

import "os"

const (
	Privileged = "PRIVILEGED_NAMESPACE"
	Sandbox    = "SANDBOX_NAMESPACE"
)

// Namespace settings.
type Namespace struct {
	Privileged string
	Sandbox    string
}

func (r *Namespace) Load() error {
	if s, found := os.LookupEnv(Sandbox); found {
		r.Sandbox = s
	}
	if s, found := os.LookupEnv(Privileged); found {
		r.Privileged = s
	}
	return nil
}
