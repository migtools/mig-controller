#!/bin/bash

# Pull mig-ui route host from disk to set CORS_ALLOWED_ORIGINS 
MIG_UI_ROUTE_PATH=$KUBECONFIG-ui-route
MIG_UI_ROUTE=$(cat $MIG_UI_ROUTE_PATH)
if [ $? -eq 0 ]; then
    echo "Found mig-ui route domain. Setting discovery service CORS_ALLOWED_ORIGINS=$MIG_UI_ROUTE"
    export CORS_ALLOWED_ORIGINS=${MIG_UI_ROUTE}
else
    echo "Missing mig-ui route domain. Continuing without setting CORS_ALLOWED_ORIGINS."
fi

# Start the controller
go run -tags "${BUILDTAGS}" ./cmd/manager/main.go