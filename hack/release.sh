#!/bin/bash

if [ "$0" != "hack/release.sh" ]; then echo "Usage: run 'hack/release.sh' from project base directory"; exit 1; fi
if [ -z "$1" ]; then echo "Usage: 'hack/release.sh RELEASE_SEMVER'"; exit 2; fi

mig_release_tag=$1

echo "Patching release image before 'kustomize build config/default'"
cat <<EOF > config/default/manager_image_patch.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: controller-manager
  namespace: mig
spec:
  template:
    spec:
      containers:
      # Change the value of image field below to your controller image URL
      - image: quay.io/ocpmigrate/mig-controller:$mig_release_tag
        name: manager
EOF

echo "Writing release manifest to 'config/releases/$mig_release_tag/manifest.yaml'"
mkdir -p config/releases/$mig_release_tag
kustomize build config/default > config/releases/$mig_release_tag/manifest.yaml

echo "Undo change to config/default/manager_image_patch.yaml"
git checkout config/default/manager_image_patch.yaml

# GitHub release flow will take care of tagging, just need a commit for it to point at
echo "Running 'git add config/releases/$mig_release_tag'"
git add config/releases/$mig_release_tag

echo "Running 'git commit -m $mig_release_tag --edit'"
git commit -m "Release $mig_release_tag" --edit

echo
echo "================= Final steps to release ====================="
echo "  1. Push the release commit that has just been created"
echo "  2. Use GitHub release flow to create release from commit"
echo "  3. Run manual build trigger on quay.io against released tag"
echo "=============================================================="
