#!/bin/bash

# Patch the MigrationController CR for the UI use the local discovery service
echo "Patching MigrationController 'spec.discovery_api_url: http://localhost:8080'. You may need to restart mig-ui for changes to take effect."
oc --namespace openshift-migration patch migrationcontroller migration-controller --type=json \
--patch '[{ "op": "add", "path": "/spec/discovery_api_url", "value": "http://localhost:8080" }]'
echo

# Patch the MigrationController CR to disable the in-cluster mig-controller deploy
echo "Patching MigrationController 'spec.migration_controller: false'. The controller will be removed when mig-operator reconciles."
oc --namespace openshift-migration patch migrationcontroller migration-controller --type=json \
--patch '[{ "op": "replace", "path": "/spec/migration_controller", "value": false }]'
echo

# Expose the registry route and grab the route URL (OCP 3)
oc expose svc docker-registry -n default
export ROUTE_CANDIDATE=$(oc get route docker-registry -n default -o jsonpath="{.spec.host}")
if [ -z "$ROUTE_CANDIDATE" ]; then
    echo "OCP 3 registry svc/route not found, assuming OCP 4"
    echo
else
    echo "Using OCP 3 registry route: ${ROUTE_CANDIDATE}"
    echo
fi


# Expose the registry route and grab the route URL (OCP 4)
if [[ -z "$ROUTE_CANDIDATE" ]]; then
    oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge && sleep 2s 
    export ROUTE_CANDIDATE=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
    if [ $? -eq 0 ]; then
        echo "Using OCP 4 registry route: ${ROUTE_CANDIDATE}"
        echo
    fi
fi
echo

# Patch the 'host' MigCluster with spec.exposedRegistryPath so local controller can perform image transfers
if [ -z "$ROUTE_CANDIDATE" ]; then
    echo "Unable to retrieve registry route. Skipping addition to 'host' MigCluster spec.exposedRegistryPath"
    exit 0
fi

echo "Patching 'host' MigCluster with spec.exposedRegistryPath=${ROUTE_CANDIDATE}"
oc --namespace openshift-migration patch migcluster host --type=json \
--patch "[{ \"op\": \"add\", \"path\": \"/spec/exposedRegistryPath\", \"value\": \"${ROUTE_CANDIDATE}\" }]"
