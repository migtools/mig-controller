#!/bin/bash

# Get the local OpenShift registry hostname
REGISTRY_HOST=$(oc registry info)

# Create a StatefulSet using the internal registry s2i image::
cat <<EOF | oc create -f -
apiVersion: v1     
kind: Service
metadata:
  name: test-statefulset
  labels:                    
    run: test-statefulset
spec:                         
  ports:
  - name: 8080-tcp
    port: 8080
    protocol: TCP
    targetPort: 8080
  clusterIP: None
  selector:
    run: test-statefulset
EOF

cat <<EOF | oc create -f -
apiVersion: apps/v1
kind: StatefulSet
metadata:
  creationTimestamp: null
  name: test-statefulset
  labels:
    run: test-statefulset
spec:
  selector:
    matchLabels:
      run: test-statefulset
  serviceName: "test-statefulset"
  template:
    metadata:
      creationTimestamp: null
      labels:
        run: test-statefulset
    spec:
      containers:
      - image: $REGISTRY_HOST/registry-example/nodejs-ex:latest
        name: test-statefulset
        resources: {}
EOF

oc expose service/test-statefulset