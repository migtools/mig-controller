#!/bin/bash

# Get the local OpenShift registry hostname
REGISTRY_HOST=$(oc registry info)

# Create a DaemonSet using the internal registry s2i image:
cat <<EOF | oc create -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  name: test-daemonset
  labels:
    run: test-daemonset
spec:
  selector:
    matchLabels:
      run: test-daemonset
  template:
    metadata:
      creationTimestamp: null
      labels:
        run: test-daemonset
    spec:
      containers:
      - image: $REGISTRY_HOST/registry-example/nodejs-ex:latest
        name: test-daemonset
        resources: {}
EOF