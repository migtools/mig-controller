#!/bin/bash

if [ "$0" != "hack/release.sh" ]; then echo "Usage: run 'hack/release.sh' from project base directory"; exit 1; fi
if [ -z "$1" ]; then echo "Usage: 'hack/release.sh RELEASE_TAG_TO_CREATE'"; exit 2; fi

mig_release_tag=$1

mkdir -p config/releases
kustomize build config/default > config/releases/$mig_release_tag.yaml
echo "Wrote release manifest 'config/releases/$mig_release_tag.yaml'"
