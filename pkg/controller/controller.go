/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"github.com/konveyor/mig-controller/pkg/controller/discovery"
	"github.com/konveyor/mig-controller/pkg/controller/migcluster"
	"github.com/konveyor/mig-controller/pkg/controller/migmigration"
	"github.com/konveyor/mig-controller/pkg/controller/migplan"
	"github.com/konveyor/mig-controller/pkg/controller/migstorage"
	"github.com/konveyor/mig-controller/pkg/settings"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

//
// Function provided by controller packages to add
// them self to the manager.
type AddFunction func(manager.Manager) error

var Controllers = map[string]AddFunction{
	settings.ClusterRole:   migcluster.Add,
	settings.DiscoveryRole: discovery.Add,
	settings.MigrationRole: migmigration.Add,
	settings.PlanRole:      migplan.Add,
	settings.StorageRole:   migstorage.Add,
	// TODO: probably need to add mighook
}

//
// Add controllers to the manager based on role.
func AddToManager(m manager.Manager) error {
	err := settings.Settings.Load()
	if err != nil {
		return err
	}

	for controllerRole, addFunc := range Controllers {
		if settings.Settings.Role.Enabled(controllerRole) {
			if err := addFunc(m); err != nil {
				return err
			}
		}
	}

	return nil
}
