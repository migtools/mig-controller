package settings

import "os"

const (
	// Proxy environment variables
	HttpProxy  = "HTTP_PROXY"
	HttpsProxy = "HTTPS_PROXY"
	NoProxy    = "NO_PROXY"
)

type Proxy struct {
	Https string
	Http  string
	None  string
}

func (r *Proxy) Load() error {
	if s, found := os.LookupEnv(HttpProxy); found {
		r.Http = s
	}
	if s, found := os.LookupEnv(HttpsProxy); found {
		r.Https = s
	}
	if s, found := os.LookupEnv(NoProxy); found {
		r.None = s
	}
	return nil
}
