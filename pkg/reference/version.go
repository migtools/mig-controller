package reference

import (
	"strconv"
	"strings"

	"k8s.io/client-go/discovery"
)

// versionUndefined undefined minor version for the cluster
const versionUndefined = -1

// AppsGap represents kubernetes version when apps/v1 (stable) was introduced
const AppsGap int = 16

// GetKubeVersion returns minor kubernetes cluster version from the server
func GetKubeVersion(discovery discovery.DiscoveryInterface) (int, error) {
	serverVersion, err := discovery.ServerVersion()
	if err != nil {
		return versionUndefined, err
	}

	minorVersion := strings.Trim(serverVersion.Minor, "+")
	kubeVersion, err := strconv.Atoi(minorVersion)
	if err != nil {
		return versionUndefined, err
	}

	return kubeVersion, nil
}
