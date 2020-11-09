#!/bin/bash
KUBECONFIG_POSTFIX="mig-controller"

if [ -z "$KUBECONFIG" ]
then
      echo "\$KUBECONFIG not set. Aborting migration-controller SA login"
      exit 1
fi

# Set path for new KUBECONFIG
MIG_KUBECONFIG=$KUBECONFIG-$KUBECONFIG_POSTFIX

# Drop a copy of mig-ui hostname on disk
MIG_UI_ROUTE_PATH=$MIG_KUBECONFIG-ui-route

# Interrogate mig-ui route to set CORS_ALLOWED_ORIGINS 
MIG_UI_ROUTE_HOST=$(oc get route migration -n openshift-migration migration -o=jsonpath='{.items[0].spec.host}')
if [ $? -eq 0 ]; then
    echo "Found mig-ui route domain."
    echo "${MIG_UI_ROUTE_HOST}" > ${MIG_UI_ROUTE_PATH}
else
    echo "Missing mig-ui route domain. Continuing without setting CORS_ALLOWED_ORIGINS."
fi

# Create copy of original KUBECONFIG that will be logged in with mig-controller SA token
cp $KUBECONFIG $MIG_KUBECONFIG

# Check for existence of migration-controller SA before logging in
KUBECONFIG=$MIG_KUBECONFIG oc get sa migration-controller -n openshift-migration > /dev/null
if [ $? -eq 0 ]; then
    echo "Found migration-controller SA. Performing token login..."
else
    echo "Missing migration-controller SA in openshift-migration namespace. Verify mig-operator installed SA successfully"
    exit 2
fi

# Login to migration controller SA without making changes to original KUBECONFIG
KUBECONFIG=$MIG_KUBECONFIG \
oc login --token $(oc sa get-token migration-controller -n openshift-migration) > /dev/null