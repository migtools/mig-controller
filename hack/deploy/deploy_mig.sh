#!/bin/bash

echo
echo "==========================="
echo "Deploying mig-controller..."
echo "==========================="
# mig-controller CRDs
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/config/crds/migration_v1alpha1_migcluster.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/config/crds/migration_v1alpha1_migmigration.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/config/crds/migration_v1alpha1_migplan.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/config/crds/migration_v1alpha1_migstorage.yaml

# cluster-registry cluster CRD
oc apply -f https://raw.githubusercontent.com/kubernetes/cluster-registry/master/cluster-registry-crd.yaml

# mig-controller deployment
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/config/releases/latest/manifest.yaml