package common

import "github.com/konveyor/mig-controller/pkg/settings"

func IsInSingletonNamespace(namespace string) bool {
	return settings.Settings.SingletonNamespace == "" || namespace == settings.Settings.SingletonNamespace
}

func IsInSandboxNamespace(namespace string) bool {
	return settings.Settings.SandboxNamespace == "" || namespace == settings.Settings.SandboxNamespace
}
