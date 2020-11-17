# Build the manager binary
FROM quay.io/konveyor/golang:1.14.4 as builder

# Copy in the go src
WORKDIR /go/src/github.com/konveyor/mig-controller
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/
ENV BUILDTAGS containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp exclude_graphdriver_overlay

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -tags "$BUILDTAGS" -a -o manager github.com/konveyor/mig-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM registry.access.redhat.com/ubi8-minimal
WORKDIR /
COPY --from=builder /go/src/github.com/konveyor/mig-controller/manager .
ENTRYPOINT ["/manager"]
