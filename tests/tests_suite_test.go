package tests

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

func TestMigmigration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var sourceClient *kubernetes.Clientset
var hostClient client.Client
var hostCfg *rest.Config
var sourceCfg *rest.Config
var dynamicClient dynamic.Interface
var controllerCR schema.GroupVersionResource
var err error
var shouldCreate bool

var _ = BeforeSuite(func() {
	hostCfg, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv(HOSTCONFIG)))
	Expect(err).ToNot(HaveOccurred())

	Expect(migapi.AddToScheme(scheme.Scheme)).Should(Succeed())

	hostClient, err = client.New(hostCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())

	sourceCfg, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv(SOURCECONFIG)))
	Expect(err).ToNot(HaveOccurred())

	sourceClient, err = kubernetes.NewForConfig(sourceCfg)
	Expect(err).ToNot(HaveOccurred())

	dynamicClient, err = dynamic.NewForConfig(hostCfg)
	Expect(err).ToNot(HaveOccurred())

	controllerCR = schema.GroupVersionResource{
		Group:    "migration.openshift.io",
		Version:  "v1alpha1",
		Resource: "migrationcontrollers",
	}

	_, err := dynamicClient.Resource(controllerCR).Namespace(MigrationNamespace).Get(context.TODO(), MigrationController, metav1.GetOptions{})
	shouldCreate = false
	if errors.IsNotFound(err) {
		shouldCreate = true
	} else {
		Expect(err).ToNot(HaveOccurred())
	}

	if shouldCreate {
		_, err := dynamicClient.Resource(controllerCR).Namespace(MigrationNamespace).Create(ctx, NewMigrationController(true, true), metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	Eventually(func() string {
		controller, err := dynamicClient.Resource(controllerCR).Namespace(MigrationNamespace).Get(context.TODO(), MigrationController, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		phase, found, err := unstructured.NestedString(controller.Object, "status", "phase")
		Expect(err).ToNot(HaveOccurred())
		if !found {
			log.Println("phase not found")
			return ""
		}
		log.Println(fmt.Sprintf("Waiting for migration-controller to be ready, current phase - %s", phase))
		return phase
	}, time.Minute*5, time.Second).Should(Equal("Reconciled"))

	token := GetMigSaToken(sourceClient)
	Expect(token).ShouldNot(BeNil())

	migCluster, secret := NewMigCluster(token)
	Expect(hostClient.Create(ctx, secret)).Should(Succeed())
	Expect(hostClient.Create(ctx, migCluster)).Should(Succeed())

	Eventually(func() bool {
		log.Println(fmt.Sprintf("Waiting for %s migCluster to be ready", E2ETestObjectName))
		Expect(hostClient.Get(ctx, client.ObjectKey{Name: E2ETestObjectName, Namespace: MigrationNamespace}, migCluster)).Should(Succeed())
		return migCluster.Status.IsReady()
	}, time.Minute*5, time.Second).Should(Equal(true))
	log.Println(fmt.Sprintf("%s migCluster is ready", E2ETestObjectName))

	migStorage, secret := NewMigStorage()
	Expect(hostClient.Create(ctx, secret)).Should(Succeed())
	Expect(hostClient.Create(ctx, migStorage)).Should(Succeed())

	Eventually(func() bool {
		log.Println(fmt.Sprintf("Waiting for %s migStorage to be ready", E2ETestObjectName))
		Expect(hostClient.Get(ctx, client.ObjectKey{Name: E2ETestObjectName, Namespace: MigrationNamespace}, migStorage)).Should(Succeed())
		return migStorage.Status.IsReady()
	}, time.Minute*5, time.Second).Should(Equal(true))
	log.Println(fmt.Sprintf("%s migStorage is ready", E2ETestObjectName))
	log.Println("Set up is complete")
}, 60)

var _ = AfterSuite(func() {
	log.Println("Cleaning up is beginning")
	ctx := context.TODO()
	err = hostClient.Delete(ctx, &migapi.MigStorage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      E2ETestObjectName,
			Namespace: MigrationNamespace,
		},
	})
	if err != nil {
		log.Println(err)
	}

	err = hostClient.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestStorageSecret,
			Namespace: ConfigNamespace,
		},
	})
	if err != nil {
		log.Println(err)
	}

	err = hostClient.Delete(ctx, &migapi.MigCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      E2ETestObjectName,
			Namespace: MigrationNamespace,
		},
	})
	if err != nil {
		log.Println(err)
	}

	err = hostClient.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestClusterSecret,
			Namespace: ConfigNamespace,
		},
	})
	if err != nil {
		log.Println(err)
	}

	if shouldCreate {
		err = dynamicClient.Resource(controllerCR).Namespace(MigrationNamespace).Delete(ctx, MigrationController, metav1.DeleteOptions{})
		if err != nil {
			log.Println(err)
		}
	}
})
