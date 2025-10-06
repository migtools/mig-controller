FROM brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_8_golang_1.24 AS builder
COPY . /workspace
WORKDIR /workspace
USER root
RUN chown -R 1001:0 /workspace
user 1001
ENV GOEXPERIMENT strictfipsruntime
ENV BUILDTAGS containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp exclude_graphdriver_overlay strictfipsruntime
WORKDIR /workspace/
RUN GOMODCACHE=/workspace/mig-controller/deps/gomod/pkg/mod CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -mod=readonly -tags "$BUILDTAGS" -a -o manager ./cmd/manager
WORKDIR /workspace/crane/
RUN GOMODCACHE=/workspace/crane/deps/gomod/pkg/mod CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -mod=readonly -tags strictfipsruntime -ldflags '-w' -o crane main.go

FROM registry.redhat.io/ubi8/ubi:latest
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/crane/crane .
COPY LICENSE /licenses/
USER 65534:65534
ENTRYPOINT ["/manager"]
