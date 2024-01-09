package directvolumemigration

import (
	"fmt"
	"net/url"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/mig-controller/pkg/settings"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konveyor/crane-lib/state_transfer/endpoint"
	cranemeta "github.com/konveyor/crane-lib/state_transfer/meta"
	cranetransport "github.com/konveyor/crane-lib/state_transfer/transport"
	stunneltransport "github.com/konveyor/crane-lib/state_transfer/transport/stunnel"
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
		return liberr.Wrap(err)
	}

	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	transportOptions, err := t.getStunnelOptions()
	if err != nil {
		return liberr.Wrap(err)
	}

	for ns := range t.getPVCNamespaceMap() {
		sourceNs := getSourceNs(ns)
		destNs := getDestNs(ns)
		nnPair := cranemeta.NewNamespacedPair(
			types.NamespacedName{Name: DirectVolumeMigrationRsyncClient, Namespace: sourceNs},
			types.NamespacedName{Name: DirectVolumeMigrationRsyncClient, Namespace: destNs},
		)

		endpoints, err := t.getEndpoints(destClient, destNs)
		if err != nil {
			return liberr.Wrap(err)
		}
		if len(endpoints) == 0 {
			continue
		}

		fsStunnelTransport, err := stunneltransport.GetTransportFromKubeObjects(
			srcClient, destClient, "fs", nnPair, endpoints[0], transportOptions)
		if err != nil && !k8serror.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		if fsStunnelTransport == nil {
			err = createStunnelForPrefix(sourceNs, destNs, "fs", transportOptions, endpoints[0], srcClient, destClient)
			if err != nil {
				return liberr.Wrap(err)
			}
		}

		blockStunnelTransport, err := stunneltransport.GetTransportFromKubeObjects(
			srcClient, destClient, "block", nnPair, endpoints[1], transportOptions)
		if err != nil && !k8serror.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		if blockStunnelTransport == nil {
			err = createStunnelForPrefix(sourceNs, destNs, "block", transportOptions, endpoints[1], srcClient, destClient)
			if err != nil {
				return liberr.Wrap(err)
			}
		}

	}

	return nil
}

func createStunnelForPrefix(sourceNs, destNs, prefix string, transportOptions *cranetransport.Options, e endpoint.Endpoint, srcClient, destClient client.Client) error {
	nsPair := cranemeta.NewNamespacedPair(
		types.NamespacedName{Namespace: sourceNs},
		types.NamespacedName{Namespace: destNs},
	)
	stunnelTransport := stunneltransport.NewTransport(nsPair, transportOptions)

	err := stunnelTransport.CreateServer(destClient, prefix, e)
	if err != nil {
		return liberr.Wrap(err)
	}

	err = stunnelTransport.CreateClient(srcClient, prefix, e)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (t *Task) getStunnelOptions() (*cranetransport.Options, error) {
	proxyConfig, err := t.generateStunnelProxyConfig()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	transportOptions := &cranetransport.Options{
		ProxyURL:      proxyConfig.ProxyHost,
		ProxyUsername: proxyConfig.ProxyUsername,
		ProxyPassword: proxyConfig.ProxyPassword,
	}
	// retrieve transfer image from source cluster
	srcCluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if srcCluster != nil {
		srcTransferImage, err := srcCluster.GetRsyncTransferImage(t.Client)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		transportOptions.StunnelClientImage = srcTransferImage
	}
	// retrieve transfer image from destination cluster
	destCluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if destCluster != nil {
		destTransferImage, err := destCluster.GetRsyncTransferImage(t.Client)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		transportOptions.StunnelServerImage = destTransferImage
	}
	return transportOptions, nil
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
