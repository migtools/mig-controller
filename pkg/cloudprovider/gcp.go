package cloudprovider

import (
	"context"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"google.golang.org/api/option"
	appsv1 "k8s.io/api/apps/v1"
	kapi "k8s.io/api/core/v1"
)

// Credentials secret.
const (
	GcpCredentials          = "gcp-credentials"
	GcpCloudSecretName      = "cloud-credentials-gcp"
	GcpCloudCredentialsPath = "credentials-gcp/cloud"
)

type GCPProvider struct {
	BaseProvider
	Bucket                  string
	KMSKeyId                string
	SnapshotCreationTimeout string
}

func (p *GCPProvider) GetCloudSecretName() string {
	return GcpCloudSecretName
}

func (p *GCPProvider) GetCloudCredentialsPath() string {
	return GcpCloudCredentialsPath
}

func (p *GCPProvider) UpdateBSL(bsl *velero.BackupStorageLocation) {
	bsl.Spec.Provider = GCP
	bsl.Spec.StorageType = velero.StorageType{
		ObjectStorage: &velero.ObjectStorageLocation{
			Bucket: p.Bucket,
			Prefix: "velero",
		},
	}
	if p.KMSKeyId != "" {
		bsl.Spec.Config["kmsKeyId"] = p.KMSKeyId
	}
}

func (p *GCPProvider) UpdateVSL(vsl *velero.VolumeSnapshotLocation) {
	vsl.Spec.Provider = GCP
	if p.SnapshotCreationTimeout != "" {
		vsl.Spec.Config["snapshotCreationTimeout"] = p.SnapshotCreationTimeout
	}
}

func (p *GCPProvider) UpdateCloudSecret(secret, cloudSecret *kapi.Secret) error {
	cloudSecret.Data = map[string][]byte{
		"cloud": secret.Data[GcpCredentials],
	}
	return nil
}

func (p *GCPProvider) UpdateRegistrySecret(secret, registrySecret *kapi.Secret) error {
	registrySecret.Data = map[string][]byte{
		"cloud": secret.Data[GcpCredentials],
	}
	return nil
}

func (p *GCPProvider) UpdateRegistryDeployment(deployment *appsv1.Deployment, name, dirName string) {
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	if envVars == nil {
		envVars = []kapi.EnvVar{}
	}
	gcpEnvVars := []kapi.EnvVar{
		{
			Name:  "REGISTRY_STORAGE",
			Value: "gcs",
		},
		{
			Name:  "REGISTRY_STORAGE_GCS_BUCKET",
			Value: p.Bucket,
		},
		{
			Name:  "REGISTRY_STORAGE_GCS_ROOTDIRECTORY",
			Value: "/" + dirName,
		},
		{
			Name:  "REGISTRY_STORAGE_GCS_KEYFILE",
			Value: "/credentials/cloud",
		},
	}
	deployment.Spec.Template.Spec.Containers[0].Env = append(envVars, gcpEnvVars...)
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []kapi.VolumeMount{
		{
			Name:      "cloud-credentials",
			MountPath: "/credentials",
		},
	}
	deployment.Spec.Template.Spec.Volumes = []kapi.Volume{
		{
			Name: "cloud-credentials",
			VolumeSource: kapi.VolumeSource{
				Secret: &kapi.SecretVolumeSource{
					SecretName: name,
				},
			},
		},
	}
}

func (p *GCPProvider) Validate(secret *kapi.Secret) []string {
	fields := []string{}

	if secret != nil {
		keySet := []string{
			GcpCredentials,
		}
		for _, k := range keySet {
			p, _ := secret.Data[k]
			if p == nil || len(p) == 0 {
				fields = append(fields, "Secret(content)")
				break
			}
		}
	}

	switch p.Role {
	case BackupStorage:
		if p.Bucket == "" {
			fields = append(fields, "Bucket")
		}
	case VolumeSnapshot:
		if p.SnapshotCreationTimeout != "" {
			_, err := time.ParseDuration(p.SnapshotCreationTimeout)
			if err != nil {
				fields = append(fields, "SnapshotCreationTimeout")
			}
		}
	}

	return fields
}

func (p *GCPProvider) Test(secret *kapi.Secret) error {
	var err error
	if secret == nil {
		return nil
	}

	switch p.Role {
	case BackupStorage:
		key, _ := uuid.NewUUID()
		test := GcsTest{
			key:    key.String(),
			bucket: p.Bucket,
			secret: secret,
		}
		err = test.Run()
		if err != nil {
			return err
		}
	case VolumeSnapshot:
		// TBD
	}

	return nil
}

type GcsTest struct {
	bucket string
	secret *kapi.Secret
	key    string
}

func (r *GcsTest) Run() error {
	client, err := r.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	err = r.upload(client)
	if err != nil {
		return err
	}
	defer r.delete(client)
	err = r.download(client)
	if err != nil {
		return err
	}

	return nil
}

func (r *GcsTest) newClient() (*storage.Client, error) {
	client, err := storage.NewClient(
		context.Background(),
		option.WithScopes(storage.ScopeReadWrite),
		option.WithCredentialsJSON(r.secret.Data[GcpCredentials]))
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (r *GcsTest) upload(client *storage.Client) error {
	bucket := client.Bucket(r.bucket)
	object := bucket.Object(r.key)
	writer := object.NewWriter(context.Background())
	_, err := writer.Write([]byte{0})
	if err != nil {
		writer.Close()
		return err
	}
	err = writer.Close()
	return err
}

func (r *GcsTest) download(client *storage.Client) error {
	bucket := client.Bucket(r.bucket)
	object := bucket.Object(r.key)
	reader, err := object.NewReader(context.Background())
	if err != nil {
		return err
	}
	_, err = reader.Read(make([]byte, 1))
	if err != nil {
		reader.Close()
		return err
	}
	err = reader.Close()
	return err
}

func (r *GcsTest) delete(client *storage.Client) error {
	bucket := client.Bucket(r.bucket)
	object := bucket.Object(r.key)
	err := object.Delete(context.Background())
	return err
}
