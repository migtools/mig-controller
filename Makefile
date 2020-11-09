# Image URL to use all building/pushing image targets
IMG ?= quay.io/ocpmigrate/mig-controller:latest
GOOS ?= `go env GOOS`
KUBECONFIG_POSTFIX ?= mig-controller
BUILDTAGS ?= containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp exclude_graphdriver_overlay

ci: all

all: test manager

# Run tests
test: generate fmt vet manifests
	go test -tags "${BUILDTAGS}" ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -tags "${BUILDTAGS}" -o bin/manager github.com/konveyor/mig-controller/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config after login as mig controller SA
run: generate fmt vet
	./hack/controller-sa-login.sh
	KUBECONFIG=$(KUBECONFIG)-${KUBECONFIG_POSTFIX} ./hack/start-local-controller.sh

# Same as `make run`, but skips generate, fmt, vet. Useful for quick iteration.
run-fast:
	./hack/controller-sa-login.sh
	KUBECONFIG=$(KUBECONFIG)-${KUBECONFIG_POSTFIX} ./hack/start-local-controller.sh

# Run against the configured Kubernetes cluster in ~/.kube/config, skip login to mig-controller SA
run-skip-sa-login: generate fmt vet
	./hack/start-local-controller.sh

tilt:
	 IMG=${IMG} TEMPLATE=${TEMPLATE} tilt up --hud=false --no-browser --file tools/tilt/Tiltfile

# Patch MigrationController CR to use local mig-controller + discovery service
use-local-controller:
	oc patch migrationcontroller migration-controller --type=json \
	--patch '[{ "op": "add", "path": "/spec/discovery_api_url_override", "value": "http://localhost:8080" }]'
	oc patch migrationcontroller migration-controller --type=json \
	--patch '[{ "op": "replace", "path": "/spec/migration_controller", "value": false }]'

# Patch MigrationController CR to use on-cluster mig-controller + discovery service
use-oncluster-controller:
	oc patch migrationcontroller migration-controller --type=json \
	--patch '[{ "op": "replace", "path": "/spec/migration_controller", "value": true }]'
	oc patch migrationcontroller migration-controller --type=json \
	--patch '[{ "op": "remove", "path": "/spec/discovery_api_url_override" }]'

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, Webhooks
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go webhook

# Copy sample CRs to a new 'migsamples' directory that is in .gitignore to avoid committing SA tokens
samples:
	mkdir -p migsamples
	cp -v config/samples/* migsamples

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet -tags "${BUILDTAGS}" ./pkg/... ./cmd/...

# Generate code
generate: conversion-gen
	go generate ./pkg/... ./cmd/...

# Generate conversion functions
conversion-gen:
	./hack/conversion-gen-${GOOS} --go-header-file ./hack/boilerplate.go.txt --output-file-base zz_conversion_generated -i ./pkg/compat/conversion/...

# Build the docker image
#docker-build: test
docker-build:
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
