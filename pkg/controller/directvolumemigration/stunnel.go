package directvolumemigration

import (
	"fmt"
	"net/url"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/mig-controller/pkg/settings"

	//"encoding/asn1"

	//"k8s.io/apimachinery/pkg/types"
	route_endpoint "github.com/konveyor/crane-lib/state_transfer/endpoint/route"
	cranemeta "github.com/konveyor/crane-lib/state_transfer/meta"
	cranetransport "github.com/konveyor/crane-lib/state_transfer/transport"
	stunnel_transport "github.com/konveyor/crane-lib/state_transfer/transport/stunnel"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type stunnelConfig struct {
	Name          string
	Namespace     string
	StunnelPort   int32
	RsyncRoute    string
	RsyncPort     int32
	VerifyCA      bool
	VerifyCALevel string
	stunnelProxyConfig
}

type stunnelProxyConfig struct {
	ProxyHost     string
	ProxyUsername string
	ProxyPassword string
}

func (t *Task) ensureStunnelTransport() error {
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}

	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}

	for ns := range t.getPVCNamespaceMap() {
		sourceNs := getSourceNs(ns)
		destNs := getDestNs(ns)
		nnPair := cranemeta.NewNamespacedPair(
			types.NamespacedName{Name: DirectVolumeMigrationRsyncClient, Namespace: sourceNs},
			types.NamespacedName{Name: DirectVolumeMigrationRsyncClient, Namespace: destNs},
		)

		endpoint, _ := route_endpoint.GetEndpointFromKubeObjects(
			destClient,
			types.NamespacedName{
				Namespace: destNs,
				Name:      DirectVolumeMigrationRsyncTransferRoute,
			},
		)
		if endpoint == nil {
			continue
		}

		stunnelTransport, err := stunnel_transport.GetTransportFromKubeObjects(
			srcClient, destClient, nnPair, endpoint)
		if err != nil && !k8serror.IsNotFound(err) {
			return liberr.Wrap(err)
		}

		if stunnelTransport == nil {
			nsPair := cranemeta.NewNamespacedPair(
				types.NamespacedName{Namespace: sourceNs},
				types.NamespacedName{Namespace: destNs},
			)
			stunnelTransport = stunnel_transport.NewTransport(nsPair, &cranetransport.Options{})

			err = stunnelTransport.CreateServer(destClient, endpoint)
			if err != nil {
				return liberr.Wrap(err)
			}

			err = stunnelTransport.CreateClient(srcClient, endpoint)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	return nil
}

// generateStunnelProxyConfig loads stunnel proxy configuration from app settings
func (t *Task) generateStunnelProxyConfig() (stunnelProxyConfig, error) {
	var proxyConfig stunnelProxyConfig
	tcpProxyString := settings.Settings.DvmOpts.StunnelTCPProxy
	if tcpProxyString != "" {
		t.Log.Info("Found TCP proxy string. Configuring Stunnel proxy.",
			"tcpProxyString", tcpProxyString)
		url, err := url.Parse(tcpProxyString)
		if err != nil {
			t.Log.Error(err, fmt.Sprintf("failed to parse %s setting", settings.TCPProxyKey))
			return proxyConfig, liberr.Wrap(err)
		}
		proxyConfig.ProxyHost = url.Host
		if url.User != nil {
			proxyConfig.ProxyUsername = url.User.Username()
			if pass, set := url.User.Password(); set {
				proxyConfig.ProxyPassword = pass
			}
		}
	}
	return proxyConfig, nil
}
