#!/bin/bash

echo
echo "================"
echo "Deploying Velero"
echo "================"
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/10-velero-crds.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/20-velero-deployment.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/30-restic-daemonset.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/40-mig-namespace.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/50-mig-sa.yaml
oc apply -f https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/manifests/60-cloud-credentials.yaml

echo
echo "===================================================="
echo "Adding cluster-admin role to mig:mig service account"
echo "===================================================="
oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:mig:mig

echo
echo "=========="
echo "Next Steps"
echo "=========="
echo " - Run 'oc sa get-token mig -n mig' to print an SA token for the mig SA, keep track of the printed token for later."
echo " - Use printed SA token to fill out MigCluster details to conne against a remote cluster."