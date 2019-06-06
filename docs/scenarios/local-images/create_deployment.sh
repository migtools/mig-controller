#!/bin/bash

# Get the local OpenShift registry hostname
REGISTRY_HOST=$(oc registry info)

# Create a Deployment using the internal registry s2i image:
cat <<EOF | oc create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  name: test-deployment
  labels:
    run: test-deployment
spec:
  selector:
    matchLabels:
      run: test-deployment
  template:
    metadata:
      creationTimestamp: null
      labels:
        run: test-deployment
    spec:
      containers:
      - image: $REGISTRY_HOST/registry-example/nodejs-ex:latest
        name: test-deployment
        resources: {}
EOF