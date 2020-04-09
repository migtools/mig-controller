# Image URL to use all building/pushing image targets
IMG ?= quay.io/ocpmigrate/mig-controller:latest
GOOS ?= `go env GOOS`

ci: all

all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/konveyor/mig-controller/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

tilt:
	 IMG=${IMG} TEMPLATE=${TEMPLATE} tilt up --hud=false --no-browser --file tools/tilt/Tiltfile

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
	go vet ./pkg/... ./cmd/...

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
