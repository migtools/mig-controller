package reference

import (
	"strconv"
	"strings"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// versionUndefined undefined minor version for the cluster
const versionUndefined = -1

// AppsGap represents kubernetes version when apps/v1 (stable) was introduced
const AppsGap = 16

// GetKubeVersion returns minor kubernetes cluster version from the server
func GetKubeVersion(c *rest.Config) (int, error) {
	// Find used Kubernetes cluster version
	discovery, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		return versionUndefined, err
	}

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
