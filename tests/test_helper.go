package tests

import (
	"context"
	"os"
	"strconv"
	"strings"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

// common constants
const (
	E2ETestObjectName  = "e2e-object"
	MigrationNamespace = migapi.OpenshiftMigrationNamespace
	TestSecretType     = "Opaque"
)

// cluster constants
const (
	TestDestinationCluster = "host"
	TestClusterSecret      = "e2eclustersecret"
)

// storage constants
const (
	TestStorageSecret = "e2estoragesecret"
	ConfigNamespace   = "openshift-config"
)

// migrationcontroller constants
const (
	MigrationController = "migration-controller"
)

// environment variables
const (
	EXPOSEDREGISTRYPATH       = "EXPOSEDREGISTRYPATH"
	SOURCEURL                 = "SOURCEURL"
	SOURCECONFIG              = "SOURCECONFIG"
	HOSTCONFIG                = "KUBECONFIG"
	VELERO_PLUGIN_FQIN        = "VELERO_PLUGIN_FQIN"
	MIG_CONTROLLER_IMAGE_FQIN = "MIG_CONTROLLER_IMAGE_FQIN"
	RSYNC_TRANSFER_IMAGE_FQIN = "RSYNC_TRANSFER_IMAGE_FQIN"
	MIG_NAMESPACE_LIMIT       = "MIG_NAMESPACE_LIMIT"
	MIG_POD_LIMIT             = "MIG_POD_LIMIT"
	CLUSTER_NAME              = "CLUSTER_NAME"
	RESTIC_TIMEOUT            = "RESTIC_TIMEOUT"
	MIGRATION_VELERO          = "MIGRATION_VELERO"
	MIG_PV_LIMIT              = "MIG_PV_LIMIT"
	VERSION                   = "VERSION"
	AWSBUCKETNAME             = "AWSBUCKETNAME"
	AWSSECRETKEY              = "AWSSECRETKEY"
	AWSACCESSKEY              = "AWSACCESSKEY"
	BACKUPSTORAGEPROVIDER     = "BACKUPSTORAGEPROVIDER"
	AWSREGION                 = "AWSREGION"
)

func NewMigMigration(name string, planRef string, quiesce bool, stage bool) *migapi.MigMigration {
	return &migapi.MigMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: MigrationNamespace,
		},
		Spec: migapi.MigMigrationSpec{
			MigPlanRef: &v1.ObjectReference{
				Namespace: MigrationNamespace,
				Name:      planRef,
			},
			Stage:       stage,
			QuiescePods: quiesce,
		},
	}
}

func NewMigPlan(namespaces []string, name string) *migapi.MigPlan {
	return &migapi.MigPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: MigrationNamespace,
		},
		Spec: migapi.MigPlanSpec{
			Namespaces: namespaces,
			SrcMigClusterRef: &v1.ObjectReference{
				Namespace: MigrationNamespace,
				Name:      E2ETestObjectName,
			},
			DestMigClusterRef: &v1.ObjectReference{
				Namespace: MigrationNamespace,
				Name:      TestDestinationCluster,
			},
			MigStorageRef: &v1.ObjectReference{
				Name:      E2ETestObjectName,
				Namespace: MigrationNamespace,
			},
		},
	}
}

func NewMigStorage() (*migapi.MigStorage, *v1.Secret) {
	return &migapi.MigStorage{
			ObjectMeta: metav1.ObjectMeta{
				Name:      E2ETestObjectName,
				Namespace: MigrationNamespace,
			},
			Spec: migapi.MigStorageSpec{
				BackupStorageConfig: migapi.BackupStorageConfig{
					CredsSecretRef: &v1.ObjectReference{
						Namespace: ConfigNamespace,
						Name:      TestStorageSecret,
					},
					AwsBucketName: os.Getenv(AWSBUCKETNAME),
					AwsRegion:     os.Getenv(AWSREGION),
				},
				BackupStorageProvider: os.Getenv(BACKUPSTORAGEPROVIDER),
				// TODO: define env variable. can these both be different?
				VolumeSnapshotProvider: os.Getenv(BACKUPSTORAGEPROVIDER),
				VolumeSnapshotConfig: migapi.VolumeSnapshotConfig{
					CredsSecretRef: &v1.ObjectReference{
						Namespace: ConfigNamespace,
						Name:      TestStorageSecret,
					},
				},
			},
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestStorageSecret,
				Namespace: ConfigNamespace,
			},
			Type: TestSecretType,
			Data: map[string][]byte{
				"aws-access-key-id":     []byte(os.Getenv(AWSACCESSKEY)),
				"aws-secret-access-key": []byte(os.Getenv(AWSSECRETKEY)),
			},
		}
}

func NewMigCluster(saToken []byte) (*migapi.MigCluster, *v1.Secret) {
	return &migapi.MigCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      E2ETestObjectName,
				Namespace: MigrationNamespace,
			},
			Spec: migapi.MigClusterSpec{
				IsHostCluster: false,
				URL:           os.Getenv(SOURCEURL),
				ServiceAccountSecretRef: &v1.ObjectReference{
					Namespace: ConfigNamespace,
					Name:      TestClusterSecret,
				},
				Insecure:            true,
				ExposedRegistryPath: os.Getenv(EXPOSEDREGISTRYPATH),
			},
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestClusterSecret,
				Namespace: ConfigNamespace,
			},
			Data: map[string][]byte{
				"saToken": saToken,
			},
			Type: TestSecretType,
		}
}

// We are assuming that the controller CR is created and controller is running
func NewMigrationController(installUi bool, installController bool) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "migration.openshift.io/v1alpha1",
			"kind":       "MigrationController",
			"metadata": map[string]interface{}{
				"name":      MigrationController,
				"namespace": MigrationNamespace,
			},
			// TODO: take variables from env
			"spec": map[string]interface{}{
				"velero_plugin_fqin":        getEnvStr(VELERO_PLUGIN_FQIN, "quay.io/konveyor/openshift-velero-plugin:latest"),
				"mig_controller_image_fqin": getEnvStr(MIG_CONTROLLER_IMAGE_FQIN, "quay.io/konveyor/mig-controller:latest"),
				"mig_namespace_limit":       getEnvInt(MIG_NAMESPACE_LIMIT, 10),
				"migration_ui":              installUi,
				"mig_pod_limit":             getEnvInt(MIG_POD_LIMIT, 100),
				"migration_controller":      installController,
				"migration_log_reader":      true,
				"olm_managed":               true,
				"cluster_name":              getEnvStr(CLUSTER_NAME, "host"),
				"restic_timeout":            getEnvStr(RESTIC_TIMEOUT, "1h"),
				"migration_velero":          getEnvBool(MIGRATION_VELERO, true),
				"rsync_transfer_image_fqin": getEnvStr(RSYNC_TRANSFER_IMAGE_FQIN, "quay.io/konveyor/rsync-transfer:latest"),
				"mig_pv_limit":              getEnvInt(MIG_PV_LIMIT, 100),
				"version":                   getEnvStr(VERSION, "latest"),
				"azure_resource_group":      "",
			},
		},
	}
}

func NewMigrationNS(ns string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
}

func GetMigSaToken(sourceClient *kubernetes.Clientset) []byte {
	ctx := context.TODO()
	sa, err := sourceClient.CoreV1().ServiceAccounts(MigrationNamespace).Get(ctx, MigrationController, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	for _, s := range sa.Secrets {
		if strings.Contains(s.Name, "token") {
			secret, err := sourceClient.CoreV1().Secrets(MigrationNamespace).Get(ctx, s.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return secret.Data["token"]
		}
	}
	return nil
}

func getEnvInt(name string, val int) int {
	t, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return val
	}
	return t
}

func getEnvStr(name string, val string) string {
	t := os.Getenv(name)
	if t == "" {
		return val
	}
	return t
}

func getEnvBool(name string, val bool) bool {
	t, err := strconv.ParseBool(os.Getenv(name))
	if err != nil {
		return val
	}
	return t
}

func NewPVC(name string, namespace string) *v1.PersistentVolumeClaim {
	storage, err := resource.ParseQuantity("2Gi")
	Expect(err).ToNot(HaveOccurred())

	return &v1.PersistentVolumeClaim{

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					"storage": storage,
				},
			},
		},
	}
}

func NewDeploymentFor41583() *appsv1.Deployment {
	res := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "longpvc-test",
			Namespace: "ocp-41583-longpvcname",
			Labels: map[string]string{
				"app": "longpvc-test",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &res,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "longpvc-test",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "longpvc-test",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "pod-test",
							ImagePullPolicy: "Always",
							Image:           "quay.io/konveyor/pvc-migrate-benchmark-helper:latest",
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "testvolume",
									MountPath: "/data/test",
								},
							},
							Env: []v1.EnvVar {
								{
									Name: "GENERATE_SAMPLE_DATA",
									Value: "N",
								},
								{
									Name: "NO_FILES",
									Value: "3",
								},
								{
									Name: "FILE_SIZE",
									Value: "10",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "testvolume",
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: "long-name-123456789011121314151617181920212223242526272829303132",
								},
							},
						},
					},
				},
			},
		},
	}
}
