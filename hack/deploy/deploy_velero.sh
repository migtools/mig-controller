#!/bin/bash

echo
echo "================"
echo "Deploying Velero"
echo "================"
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/00-mig-namespace.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/10-velero-crds.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/20-velero-deployment.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/30-restic-daemonset.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/50-mig-sa.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/60-cloud-credentials.yaml

echo
echo "===================================================="
echo "Adding cluster-admin role to migration:mig service account"
echo "===================================================="
oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:migration:mig

echo
echo "===================================================="
echo "Adding privileged scc to velero service account"
echo "===================================================="
oc adm policy add-scc-to-user privileged system:serviceaccount:migration:velero
