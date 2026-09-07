FROM golang:1.26.6-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev git

# Set up workspace
ENV APP_ROOT=/opt/app-root
WORKDIR $APP_ROOT/src/github.com/konveyor/mig-controller
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY go.mod go.mod
COPY go.sum go.sum
ENV BUILDTAGS containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp exclude_graphdriver_overlay
RUN CGO_ENABLED=1 GOOS=linux go build -tags "$BUILDTAGS" -a -o $APP_ROOT/src/manager github.com/konveyor/mig-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM registry.access.redhat.com/ubi8-minimal
WORKDIR /
COPY --from=builder /opt/app-root/src/manager .
USER 65534:65534
ENTRYPOINT ["/manager"]
