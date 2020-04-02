#!/bin/bash
KUBECONFIG_POSTFIX="mig-controller"

if [ -z "$KUBECONFIG" ]
then
      echo "\$KUBECONFIG not set. Aborting migration-controller SA login"
      exit 1
fi

# Create copy of original KUBECONFIG that will be logged in with mig-controller SA token
MIG_KUBECONFIG=$KUBECONFIG-$KUBECONFIG_POSTFIX
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