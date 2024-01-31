# Image URL to use all building/pushing image targets
IMG ?= quay.io/ocpmigrate/mig-controller:latest
GOOS ?= `go env GOOS`
GOBIN ?= ${GOPATH}/bin
GO111MODULE = auto
KUBECONFIG_POSTFIX ?= mig-controller
export BUILDTAGS ?= containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp exclude_graphdriver_overlay

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
	hack/use-local-controller.sh

# Patch MigrationController CR to use on-cluster mig-controller + discovery service
use-oncluster-controller:
	hack/use-oncluster-controller.sh

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -


CRD_OPTIONS ?= crdVersions=v1

# Generate manifests e.g. CRD, Webhooks, requires kubebuilder 0.13.0 or newer
manifests:
	${CONTROLLER_GEN} "crd:${CRD_OPTIONS}" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crds/bases 
	mv config/crds/bases/migration.openshift.io*yaml config/crds
	rm -rf config/crds/bases

# Copy sample CRs to a new 'migsamples' directory that is in .gitignore to avoid committing SA tokens
samples:
	mkdir -p migsamples
	cp -v config/samples/* migsamples

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet -tags "${BUILDTAGS}" -structtag=false ./pkg/... ./cmd/...

# Generate code
generate: conversion-gen controller-gen
	${CONTROLLER_GEN} object:headerFile="./hack/boilerplate.go.txt" paths="./..."

# Generate conversion functions
conversion-gen:  conversion-gen-dl
	${CONVERSION_GEN} --go-header-file ./hack/boilerplate.go.txt --output-file-base zz_conversion_generated -i github.com/konveyor/mig-controller/pkg/compat/conversion/...

# Build the docker image
#docker-build: test
docker-build:
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download controller-gen
# download controller-gen if necessary
conversion-gen-dl:
ifeq (, $(shell which conversion-gen))
	@{ \
	set -e ;\
	CONVERSION_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONVERSION_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install k8s.io/code-generator/cmd/conversion-gen@v0.19.16 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONVERSION_GEN=$(GOBIN)/conversion-gen
else
CONVERSION_GEN=$(shell which conversion-gen)
endif

# run e2e test suite
e2e-test:
	ginkgo tests/
