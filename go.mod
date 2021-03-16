module github.com/konveyor/mig-controller

go 1.14

require (
	cloud.google.com/go/storage v1.10.0
	github.com/Azure/azure-sdk-for-go v48.1.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.10
	github.com/Azure/go-autorest/autorest/adal v0.9.5
	github.com/aws/aws-sdk-go v1.35.23
	github.com/containers/image/v5 v5.7.0
	github.com/deckarep/golang-set v1.7.1
	github.com/dnaeon/go-vcr v1.1.0 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20201021153353-00ad82a08272 // indirect
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.6.3
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.3.0
	github.com/google/uuid v1.1.2
	github.com/joho/godotenv v1.3.0
	github.com/konveyor/controller v0.2.1
	github.com/konveyor/openshift-velero-plugin v0.0.0-20201023200114-f5883b430041
	github.com/mattn/go-sqlite3 v1.14.4
	github.com/onsi/gomega v1.7.0
	github.com/openshift/api v0.0.0-20200210091934-a0e53e94816b
	github.com/openshift/library-go v0.0.0-20200521120150-e4959e210d3a
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/vmware-tanzu/velero v1.4.2
	go.opencensus.io v0.22.5 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	google.golang.org/api v0.35.0
	google.golang.org/genproto v0.0.0-20201106154455-f9bfe239b0ba // indirect
	google.golang.org/grpc v1.33.2 // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/utils v0.0.0-20191218082557-f07c713de883
	sigs.k8s.io/controller-runtime v0.1.11
)

// Use fork
replace bitbucket.org/ww/goautoneg v0.0.0-20120707110453-75cd24fc2f2c => github.com/markusthoemmes/goautoneg v0.0.0-20190713162725-c6008fefa5b1

replace github.com/vmware-tanzu/velero => github.com/konveyor/velero v0.0.0-20201026230312-8bd8ce8744d5

//k8s deps pinning
replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20181213151034-8d9ed539ba31

replace k8s.io/api => k8s.io/api v0.0.0-20181213150558-05914d821849

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20181213153335-0fe22c71c476

//openshift deps pinning
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190716152234-9ea19f9dd578
