#!/bin/bash

# Patch the MigrationController CR for the UI use the in-cluster discovery service
echo "Patching MigrationController to remove overridden 'spec.discovery_api_url'. You may need to restart mig-ui for changes to take effect."
oc --namespace openshift-migration patch migrationcontroller migration-controller --type=json \
--patch '[{ "op": "remove", "path": "/spec/discovery_api_url" }]'
echo

# Patch the MigrationController CR to enable the in-cluster mig-controller deploy
echo "Patching MigrationController 'spec.migration_controller: true'. The controller will be deployed when mig-operator reconciles."
oc --namespace openshift-migration patch migrationcontroller migration-controller --type=json \
--patch '[{ "op": "replace", "path": "/spec/migration_controller", "value": true }]'
echo