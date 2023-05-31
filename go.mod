module github.com/konveyor/mig-controller

go 1.14

require (
	cloud.google.com/go/storage v1.10.0
	github.com/Azure/azure-sdk-for-go v61.4.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.24
	github.com/Azure/go-autorest/autorest/adal v0.9.18
	github.com/HdrHistogram/hdrhistogram-go v1.1.0 // indirect
	github.com/aws/aws-sdk-go v1.35.23
	github.com/containers/image/v5 v5.17.0
	github.com/deckarep/golang-set v1.7.1
	github.com/dnaeon/go-vcr v1.1.0 // indirect
	github.com/elazarl/goproxy v0.0.0-20201021153353-00ad82a08272 // indirect
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.7.7
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0
	github.com/go-playground/validator/v10 v10.10.0 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/google/uuid v1.3.0
	github.com/joho/godotenv v1.3.0
	github.com/konveyor/controller v0.4.1
	github.com/konveyor/crane-lib v0.0.11-0.20230531133520-328e74511762
	github.com/konveyor/openshift-velero-plugin v0.0.0-20210729141849-876132e34f3d
	github.com/mattn/go-sqlite3 v1.14.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v0.0.0-20210625082935-ad54d363d274
	github.com/openshift/library-go v0.0.0-20200521120150-e4959e210d3a
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/ugorji/go v1.2.6 // indirect
	github.com/vmware-tanzu/velero v1.7.1
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20220131195533-30dcbda58838 // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	google.golang.org/api v0.56.0
	k8s.io/api v0.22.14
	k8s.io/apimachinery v0.22.14
	k8s.io/client-go v0.22.14
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-runtime v0.10.2
)

// CVE-2020-28483
replace github.com/gin-gonic/gin => github.com/gin-gonic/gin v1.7.7

// CVE-2015-3627
replace github.com/docker/docker => github.com/docker/docker v20.10.14+incompatible

// CVE-2022-23648, CVE-2021-43816, CVE-2022-31030, and Ambiguous OCI manifest parsing (no CVE)
replace github.com/containerd/containerd => github.com/containerd/containerd v1.5.13

// CVE-2021-43784, CVE-2022-29162
replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.2

// OCI Manifest Type Confusion Issue (No CVE)
replace github.com/docker/distribution => github.com/docker/distribution v2.8.1+incompatible

// CVE-2021-41190
replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20211123152302-43a7dee1ec31

// CVE-2021-3121
replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2

replace k8s.io/client-go => k8s.io/client-go v0.22.14

replace k8s.io/apimachinery => k8s.io/apimachinery v0.22.14

replace k8s.io/api => k8s.io/api v0.22.14

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.14

replace k8s.io/apiserver => k8s.io/apiserver v0.22.14

// Use fork
replace bitbucket.org/ww/goautoneg v0.0.0-20120707110453-75cd24fc2f2c => github.com/markusthoemmes/goautoneg v0.0.0-20190713162725-c6008fefa5b1

replace github.com/vmware-tanzu/velero => github.com/konveyor/velero v0.10.2-0.20220124204642-f91d69bb9a5e

//k8s deps pinning

//replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93

//replace k8s.io/client-go => k8s.io/client-go v0.0.0-20181213151034-8d9ed539ba31

//replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20181213153335-0fe22c71c476

//openshift deps pinning
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190716152234-9ea19f9dd578

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.9.7
