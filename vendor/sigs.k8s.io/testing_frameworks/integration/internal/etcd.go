package internal

<<<<<<< HEAD
import (
	"net/url"
)
=======
import "net/url"
>>>>>>> cbc9bb05... fixup add vendor back

var EtcdDefaultArgs = []string{
	"--listen-peer-urls=http://localhost:0",
	"--advertise-client-urls={{ if .URL }}{{ .URL.String }}{{ end }}",
	"--listen-client-urls={{ if .URL }}{{ .URL.String }}{{ end }}",
	"--data-dir={{ .DataDir }}",
}

func DoEtcdArgDefaulting(args []string) []string {
	if len(args) != 0 {
		return args
	}

	return EtcdDefaultArgs
}

func isSecureScheme(scheme string) bool {
	// https://github.com/coreos/etcd/blob/d9deeff49a080a88c982d328ad9d33f26d1ad7b6/pkg/transport/listener.go#L53
	if scheme == "https" || scheme == "unixs" {
		return true
	}
	return false
}

func GetEtcdStartMessage(listenUrl url.URL) string {
	if isSecureScheme(listenUrl.Scheme) {
		// https://github.com/coreos/etcd/blob/a7f1fbe00ec216fcb3a1919397a103b41dca8413/embed/serve.go#L167
<<<<<<< HEAD
		return "serving client requests on "
	}

	// https://github.com/coreos/etcd/blob/a7f1fbe00ec216fcb3a1919397a103b41dca8413/embed/serve.go#L124
	return "serving insecure client requests on "
=======
		return "serving client requests on " + listenUrl.Hostname()
	}

	// https://github.com/coreos/etcd/blob/a7f1fbe00ec216fcb3a1919397a103b41dca8413/embed/serve.go#L124
	return "serving insecure client requests on " + listenUrl.Hostname()
>>>>>>> cbc9bb05... fixup add vendor back
}
