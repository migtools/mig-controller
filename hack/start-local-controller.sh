#!/bin/bash

# Warn user if in-cluster controller is running
INCLUSTER_CONTROLLER_ENABLED=$(oc get migrationcontroller migration-controller -n openshift-migration -o jsonpath='{.spec.migration_controller}')
if [ "$INCLUSTER_CONTROLLER_ENABLED" == "true" ]; then
    echo
    echo "[!] WARNING: migrationcontroller CR has '.spec.migration_controller=true' Running a local controller will conflict."
    echo "[!] To resolve conflict, run 'make use-local-controller' to disable the on-cluster controller, then re-run this command."
    echo
fi

# Pull mig-ui route host from disk to set CORS_ALLOWED_ORIGINS 
MIG_UI_ROUTE_PATH=$KUBECONFIG-ui-route
LOCAL_UI="localhost"
MIG_UI_ROUTE=$(cat $MIG_UI_ROUTE_PATH)
if [ $? -eq 0 ]; then
    echo "Found mig-ui route domain. Setting discovery service CORS_ALLOWED_ORIGINS=${MIG_UI_ROUTE},${LOCAL_UI}"
    export CORS_ALLOWED_ORIGINS="${MIG_UI_ROUTE},${LOCAL_UI}"
    rm $MIG_UI_ROUTE_PATH
else
    echo "Missing mig-ui route domain. Setting discovery service CORS_ALLOWED_ORIGINS=${LOCAL_UI}"
    export CORS_ALLOWED_ORIGINS="${LOCAL_UI}"
fi

# Start the controller
go run -tags "${BUILDTAGS}" ./cmd/manager/main.go