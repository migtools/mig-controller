if [[ -z "$KUBECONFIG" ]]; then
  echo "please export KUBECONFIG env var"
  exit 1
fi

if [[ -z "$KUBEADMIN_PASSWORD" ]]; then
  echo "please export KUBEADMIN_PASSWORD env var"
  exit 1
fi

## check if it is a 4.x cluster
if oc get oauth cluster > /dev/null;  then
  echo "check 4.x cluster verified"
else
  echo "not a 4.x cluster, please export correct kubeconfig";
  exit 1
fi

htpasswd -c -B -b users.htpasswd bob bob
htpasswd -B -b users.htpasswd alice alice
oc create secret generic htpass-secret --from-file=htpasswd=./users.htpasswd -n openshift-config
oc patch oauth cluster --type='json' -p '[ { "op": "replace", "path": "/spec", "value": {"identityProviders": [{"htpasswd": {"fileData": {"name": "htpass-secret"}}, "mappingMethod": "claim", "name": "bob_httpassd_provider", "type": "HTPasswd"}]}}]'
sleep 2
oc login -u bob -p bob
oc new-project bob-destination
oc login -u kubeadmin -p "$KUBEADMIN_PASSWORD"
cat <<EOF | oc create -f -
apiVersion: migration.openshift.io/v1alpha1
kind: MigrationTenant
metadata:
  name: migration-controller
  namespace: bob-destination
spec:
  migration_controller: "true"
  migration_ui: "true"
  olm_managed: true
  version: 1.0 (OLM)
EOF

cat <<EOF | oc apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: openshift-migration
  name: use-host-cluster
rules:
- apiGroups: ["migration.openshift.io"]
  resources: ["migclusters"]
  verbs: ["use"]
  resourceNames: ["host"]
EOF

oc create rolebinding bob-use-host --role use-cluster --user bob   --namespace openshift-migration

rm users.htpasswd