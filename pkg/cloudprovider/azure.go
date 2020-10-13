package cloudprovider

import (
	"bytes"
	"context"
	"strings"
	"time"

	storagemgmt "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2018-02-01/storage"
	azstorage "github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appsv1 "k8s.io/api/apps/v1"
	kapi "k8s.io/api/core/v1"
)

// Credentials secret.
const (
	AzureCredentials          = "azure-credentials"
	AzureCloudSecretName      = "azure-cloud-credentials"
	AzureCloudCredentialsPath = "credentials-azure/cloud"

	tenantIDKey             = "AZURE_TENANT_ID"
	subscriptionIDKey       = "AZURE_SUBSCRIPTION_ID"
	clientIDKey             = "AZURE_CLIENT_ID"
	clientSecretKey         = "AZURE_CLIENT_SECRET"
	cloudNameKey            = "AZURE_CLOUD_NAME"
	clusterResourceGroupKey = "AZURE_RESOURCE_GROUP"
)

// Registry Credentials Secret
const (
	storageAccountKeyConfigKey = "storageAccountKey"
)

type AzureProvider struct {
	BaseProvider
	StorageAccount          string
	StorageContainer        string
	ResourceGroup           string
	ClusterResourceGroup    string
	APITimeout              string
	SnapshotCreationTimeout string
}

func (p *AzureProvider) GetCloudSecretName() string {
	return AzureCloudSecretName
}

func (p *AzureProvider) GetCloudCredentialsPath() string {
	return AzureCloudCredentialsPath
}
func (p *AzureProvider) UpdateBSL(bsl *velero.BackupStorageLocation) {
	bsl.Spec.Provider = Azure
	bsl.Spec.StorageType = velero.StorageType{
		ObjectStorage: &velero.ObjectStorageLocation{
			Bucket: p.StorageContainer,
			Prefix: "velero",
		},
	}

	bsl.Spec.Config = map[string]string{
		"resourceGroup":  p.ResourceGroup,
		"storageAccount": p.StorageAccount,
	}
}

func (p *AzureProvider) UpdateVSL(vsl *velero.VolumeSnapshotLocation) {
	vsl.Spec.Provider = Azure

	vsl.Spec.Config = map[string]string{
		"resourceGroup": p.ResourceGroup,
		"apiTimeout":    p.APITimeout,
	}
	if p.SnapshotCreationTimeout != "" {
		vsl.Spec.Config["snapshotCreationTimeout"] = p.SnapshotCreationTimeout
	}
}

func (p *AzureProvider) UpdateCloudSecret(secret, cloudSecret *kapi.Secret) error {
	cloudCredsMap, err := godotenv.Unmarshal(string(secret.Data[AzureCredentials]))
	if err != nil {
		return err
	}
	if p.ClusterResourceGroup != "" {
		cloudCredsMap[clusterResourceGroupKey] = p.ClusterResourceGroup
	}
	cloudCredsEnv, err := godotenv.Marshal(cloudCredsMap)
	if err != nil {
		return err
	}
	cloudSecret.Data = map[string][]byte{
		"cloud": []byte(cloudCredsEnv),
	}
	return nil
}

func (p *AzureProvider) UpdateRegistrySecret(secret, registrySecret *kapi.Secret) error {
	// 1. Parse AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID
	cloudCredsMap, err := godotenv.Unmarshal(string(secret.Data[AzureCredentials]))
	if err != nil {
		return err
	}

	// 2. Ask Azure API for Storage Account Key
	storageAccountKey, _, err := p.getStorageAccountKey(cloudCredsMap)
	if err != nil {
		return err
	}

	// 3. Set the StorageAccountKey on the registry secret
	registrySecret.Data = map[string][]byte{
		storageAccountKeyConfigKey: []byte(storageAccountKey),
	}
	return nil
}

func (p *AzureProvider) UpdateRegistryDC(dc *appsv1.Deployment, name, dirName string) {
	envVars := dc.Spec.Template.Spec.Containers[0].Env
	if envVars == nil {
		envVars = []kapi.EnvVar{}
	}
	azureEnvVars := []kapi.EnvVar{
		{
			Name:  "REGISTRY_STORAGE",
			Value: "azure",
		},
		{
			Name:  "REGISTRY_STORAGE_AZURE_CONTAINER",
			Value: p.StorageContainer,
		},
		{
			Name:  "REGISTRY_STORAGE_AZURE_ACCOUNTNAME",
			Value: p.StorageAccount,
		},
		{
			Name: "REGISTRY_STORAGE_AZURE_ACCOUNTKEY",
			ValueFrom: &kapi.EnvVarSource{
				SecretKeyRef: &kapi.SecretKeySelector{
					LocalObjectReference: kapi.LocalObjectReference{Name: name},
					Key:                  storageAccountKeyConfigKey,
				},
			},
		},
	}
	dc.Spec.Template.Spec.Containers[0].Env = append(envVars, azureEnvVars...)
}

func (p *AzureProvider) Validate(secret *kapi.Secret) []string {
	fields := []string{}

	if secret != nil {
		keySet := []string{
			AzureCredentials,
		}
		for _, k := range keySet {
			p, _ := secret.Data[k]
			if p == nil || len(p) == 0 {
				fields = append(fields, "Secret(content)")
				break
			}
		}

		// Ensure 'azure-credentials' contains all needed vars:
		// AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_SUBSCRIPTION_ID
		cloudCreds, err := godotenv.Unmarshal(string(secret.Data[AzureCredentials]))
		if err != nil {
			return fields
		}
		if cloudCreds[tenantIDKey] == "" {
			fields = append(fields, tenantIDKey)
		}
		if cloudCreds[clientIDKey] == "" {
			fields = append(fields, clientIDKey)
		}
		if cloudCreds[clientSecretKey] == "" {
			fields = append(fields, clientSecretKey)
		}
		if cloudCreds[subscriptionIDKey] == "" {
			fields = append(fields, subscriptionIDKey)
		}
	}

	if p.ResourceGroup == "" {
		fields = append(fields, "ResourceGroup")
	}

	switch p.Role {
	case BackupStorage:
		if p.StorageAccount == "" {
			fields = append(fields, "StorageAccount")
		}
		if p.StorageContainer == "" {
			fields = append(fields, "StorageContainer")
		}
	case VolumeSnapshot:
		if p.APITimeout == "" {
			fields = append(fields, "APITimeout")
		}
		if p.SnapshotCreationTimeout != "" {
			_, err := time.ParseDuration(p.SnapshotCreationTimeout)
			if err != nil {
				fields = append(fields, "SnapshotCreationTimeout")
			}
		}
	}

	return fields
}

func (p *AzureProvider) Test(secret *kapi.Secret) error {
	var err error

	if secret == nil {
		return nil
	}

	switch p.Role {
	case BackupStorage:
		cloudCreds, err := godotenv.Unmarshal(string(secret.Data[AzureCredentials]))
		if err != nil {
			return err
		}

		storageAccountKey, azureEnv, err := p.getStorageAccountKey(cloudCreds)
		if err != nil {
			return err
		}

		key, _ := uuid.NewUUID()
		test := AzureBlobTest{
			key:               key.String(),
			container:         p.StorageContainer,
			storageAccount:    p.StorageAccount,
			storageAccountKey: storageAccountKey,
			azureEnv:          azureEnv,
		}
		err = test.Run()
	}

	return err
}

func (p *AzureProvider) getAzureEnvironment(cloudName string) (*azure.Environment, error) {
	if cloudName == "" {
		return &azure.PublicCloud, nil
	}

	env, err := azure.EnvironmentFromName(cloudName)
	return &env, err
}

func (p *AzureProvider) newServicePrincipalToken(tenantID, clientID, clientSecret string, env *azure.Environment) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, err
	}

	return adal.NewServicePrincipalToken(
		*oauthConfig,
		clientID,
		clientSecret,
		env.ResourceManagerEndpoint,
	)
}

func (p *AzureProvider) getStorageAccountKey(azureCreds map[string]string) (string, *azure.Environment, error) {
	// 1. Get Azure cloud from AZURE_CLOUD_NAME, if it exists. If the env var does not
	// exist, parseAzureEnvironment will return azure.PublicCloud.
	env, err := p.getAzureEnvironment(azureCreds[cloudNameKey])
	if err != nil {
		return "", nil, err
	}

	// 2. Set Azure subscription ID
	subscriptionID := azureCreds[subscriptionIDKey]

	// 3. Get Service Principal Token (SPT)
	spt, err := p.newServicePrincipalToken(
		azureCreds[tenantIDKey],
		azureCreds[clientIDKey],
		azureCreds[clientSecretKey],
		env,
	)
	if err != nil {
		return "", env, err
	}

	// 4. Get storageAccountsClient
	storageAccountsClient := storagemgmt.NewAccountsClientWithBaseURI(
		env.ResourceManagerEndpoint,
		subscriptionID,
	)
	storageAccountsClient.Authorizer = autorest.NewBearerAuthorizer(spt)

	// 5. Get storage key
	res, err := storageAccountsClient.ListKeys(context.TODO(), p.ResourceGroup, p.StorageAccount)
	if err != nil {
		return "", env, errors.WithStack(err)
	}
	if res.Keys == nil || len(*res.Keys) == 0 {
		return "", env, errors.New("No storage keys found")
	}

	var storageKey string
	for _, key := range *res.Keys {
		// uppercase both strings for comparison because the ListKeys call returns e.g. "FULL" but
		// the storagemgmt.Full constant in the SDK is defined as "Full".
		if strings.ToUpper(string(key.Permissions)) == strings.ToUpper(string(storagemgmt.Full)) {
			storageKey = *key.Value
			break
		}
	}

	if storageKey == "" {
		return "", env, errors.New("No storage key with Full permissions found")
	}

	return storageKey, env, nil
}

type AzureBlobTest struct {
	key               string
	container         string
	storageAccount    string
	storageAccountKey string
	azureEnv          *azure.Environment
	client            *azstorage.BlobStorageClient
}

func (r *AzureBlobTest) Run() error {
	client, err := r.getBlobClient()
	if err != nil {
		return err
	}
	r.client = client

	err = r.upload()
	if err != nil {
		return err
	}
	defer r.delete()

	err = r.download()
	if err != nil {
		return err
	}

	return nil
}

func (r *AzureBlobTest) upload() error {
	blob, err := r.getBlob()
	if err != nil {
		return err
	}

	blob.CreateBlockBlobFromReader(bytes.NewReader([]byte{0}), nil)

	return err
}

func (r *AzureBlobTest) download() error {
	blob, err := r.getBlob()
	if err != nil {
		return err
	}

	_, err = blob.Get(nil)

	return err
}

func (r *AzureBlobTest) delete() error {
	blob, err := r.getBlob()
	if err != nil {
		return err
	}
	err = blob.Delete(nil)

	return err
}

func (r *AzureBlobTest) getBlobClient() (*azstorage.BlobStorageClient, error) {
	storageClient, err := azstorage.NewBasicClientOnSovereignCloud(
		r.storageAccount,
		r.storageAccountKey,
		*r.azureEnv,
	)
	if err != nil {
		return nil, err
	}

	blobClient := storageClient.GetBlobService()
	return &blobClient, nil
}

func (r *AzureBlobTest) getBlob() (*azstorage.Blob, error) {
	container := r.client.GetContainerReference(r.container)
	if container == nil {
		return nil, errors.Errorf("unable to get container reference for bucket %v", r.container)
	}

	blob := container.GetBlobReference(r.key)
	if blob == nil {
		return nil, errors.Errorf("unable to get blob reference for key %v", r.key)
	}

	return blob, nil
}
