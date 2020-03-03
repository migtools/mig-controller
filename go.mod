module github.com/konveyor/mig-controller

go 1.14

require (
	cloud.google.com/go v0.38.0
	github.com/Azure/azure-sdk-for-go v34.1.0+incompatible
	github.com/Azure/go-autorest/autorest v0.10.0
	github.com/Azure/go-autorest/autorest/adal v0.8.2
	github.com/Azure/go-autorest/autorest/to v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30 // indirect
	github.com/aws/aws-sdk-go v1.21.4
	github.com/dnaeon/go-vcr v1.0.1 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20200220113713-29f9e0ba54ea // indirect
	// github.com/fsnotify/fsnotify v1.4.7
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gin-contrib/cors v1.3.0
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.4.0
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/joho/godotenv v1.3.0
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/mattn/go-sqlite3 v1.13.0
	github.com/onsi/gomega v1.8.1
	github.com/openshift/api v0.0.0-20190322043348-8741ff068a47
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cobra v0.0.5 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/ugorji/go v1.1.7 // indirect
	github.com/vmware-tanzu/velero v1.2.0
	go.opencensus.io v0.22.1 // indirect
	go.uber.org/zap v1.14.0 // indirect
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/api v0.10.0
	google.golang.org/genproto v0.0.0-20190927181202-20e1ac93f88c // indirect
	google.golang.org/grpc v1.24.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.0.0-20181204000039-89a74a8d264d
	k8s.io/apiextensions-apiserver v0.0.0-20181204003920-20c909e7c8c3 // indirect
	k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93
	k8s.io/apiserver v0.0.0-20181204001702-9caa0299108f
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/cluster-registry v0.0.6
	k8s.io/code-generator v0.17.0
	k8s.io/gengo v0.0.0-20190822140433-26a664648505 // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/kube-openapi v0.0.0-20200204173128-addea2498afe // indirect
	sigs.k8s.io/controller-runtime v0.1.10
	sigs.k8s.io/controller-tools v0.1.10
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace (
	// gopkg.in/fsnotify.v1 v1.4.7 => github.com/fsnotify/fsnotify v1.4.7
	github.com/vmware-tanzu/velero v0.0.0-53b3e5029c9d0ce8c6bbca305f991af0031dbd51 => github.com/konveyor/velero v0.10.2-0.20200225172845-53b3e5029c9d
	k8s.io/api => k8s.io/api v0.0.0-20181204000039-89a74a8d264d
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93
	k8s.io/client-go => k8s.io/client-go v10.0.0+incompatible
)
