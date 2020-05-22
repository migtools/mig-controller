if [[ -z "$KUBECONFIG" ]]; then
  echo "please export KUBECONFIG env var"
  exit 1
fi

export KUBEADMIN_KUBECONFIG="$KUBECONFIG-admin"
cp $KUBECONFIG $KUBEADMIN_KUBECONFIG

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

# create project for bob
oc login -u bob -p bob
oc new-project bob-destination

#create project for alice
oc login -u alice -p alice
oc new-project alice-destination

## create MigrationTenant CR for bob and alice
export KUBECONFIG=$KUBEADMIN_KUBECONFIG

# back to system admin
oc login -u system:admin
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

cat <<EOF | oc create -f -
apiVersion: migration.openshift.io/v1alpha1
kind: MigrationTenant
metadata:
  name: migration-controller
  namespace: alice-destination
spec:
  migration_controller: "true"
  migration_ui: "true"
  olm_managed: true
  version: 1.0 (OLM)
EOF

# setup bob for using host cluster
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
rm $KUBEADMIN_KUBECONFIG
echo "bob is set up to migrate to host cluster, alice should not be able to"