FROM registry.access.redhat.com/ubi8/go-toolset:1.14.7 AS builder

# Copy in the go src
ENV GOPATH=$APP_ROOT
COPY pkg    $APP_ROOT/src/github.com/konveyor/mig-controller/pkg
COPY cmd    $APP_ROOT/src/github.com/konveyor/mig-controller/cmd
COPY vendor $APP_ROOT/src/github.com/konveyor/mig-controller/vendor
ENV BUILDTAGS containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp exclude_graphdriver_overlay

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -tags "$BUILDTAGS" -a -o manager github.com/konveyor/mig-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM registry.access.redhat.com/ubi8-minimal
WORKDIR /
COPY --from=builder /opt/app-root/src/manager .
ENTRYPOINT ["/manager"]