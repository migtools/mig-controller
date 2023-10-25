package cloudprovider

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
	"github.com/konveyor/mig-controller/pkg/settings"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appsv1 "k8s.io/api/apps/v1"
	kapi "k8s.io/api/core/v1"
)

var Settings = &settings.Settings

// Credentials Secret.
const (
	AwsAccessKeyId          = "aws-access-key-id"
	AwsSecretAccessKey      = "aws-secret-access-key"
	AwsCloudSecretName      = "cloud-credentials"
	AwsCloudCredentialsPath = "credentials/cloud"
)

// S3 constants
const (
	AwsS3DefaultRegion = "us-east-1"
)

// Velero cloud-secret.
var AwsCloudCredentialsTemplate = `
[default]
aws_access_key_id=%s
aws_secret_access_key=%s
`

type AWSProvider struct {
	BaseProvider
	Bucket                  string
	Region                  string
	S3URL                   string
	PublicURL               string
	KMSKeyId                string
	SignatureVersion        string
	S3ForcePathStyle        bool
	CustomCABundle          []byte
	SnapshotCreationTimeout string
	Insecure                bool
}

func (p *AWSProvider) GetURL() string {
	if p.S3URL != "" {
		return p.S3URL
	}
	if p.PublicURL != "" {
		return p.PublicURL
	}

	return ""
}

func (p *AWSProvider) GetCloudSecretName() string {
	return AwsCloudSecretName
}

func (p *AWSProvider) GetCloudCredentialsPath() string {
	return AwsCloudCredentialsPath
}
func (p *AWSProvider) UpdateBSL(bsl *velero.BackupStorageLocation) {
	bsl.Spec.Provider = AWS
	bsl.Spec.StorageType = velero.StorageType{
		ObjectStorage: &velero.ObjectStorageLocation{
			Bucket: p.Bucket,
			Prefix: "velero",
		},
	}
	if len(p.CustomCABundle) > 0 {
		bsl.Spec.StorageType.ObjectStorage.CACert = p.CustomCABundle
	}
	bsl.Spec.Config = map[string]string{
		"s3ForcePathStyle":      strconv.FormatBool(p.GetForcePathStyle()),
		"region":                p.GetRegion(),
		"insecureSkipTLSVerify": strconv.FormatBool(p.Insecure),
	}
	if p.S3URL != "" {
		bsl.Spec.Config["s3Url"] = p.S3URL
	}
	if p.PublicURL != "" {
		bsl.Spec.Config["publicUrl"] = p.PublicURL
	}
	if p.KMSKeyId != "" {
		bsl.Spec.Config["kmsKeyId"] = p.KMSKeyId
	}
	if p.SignatureVersion != "" {
		bsl.Spec.Config["signatureVersion"] = p.SignatureVersion
	}
}

func (p *AWSProvider) UpdateVSL(vsl *velero.VolumeSnapshotLocation) {
	vsl.Spec.Provider = AWS
	vsl.Spec.Config = map[string]string{
		"region": p.GetRegion(),
	}
	if p.SnapshotCreationTimeout != "" {
		vsl.Spec.Config["snapshotCreationTimeout"] = p.SnapshotCreationTimeout
	}
}

func (p *AWSProvider) UpdateCloudSecret(secret, cloudSecret *kapi.Secret) error {
	cloudSecret.Data = map[string][]byte{
		"cloud": []byte(
			fmt.Sprintf(
				AwsCloudCredentialsTemplate,
				secret.Data[AwsAccessKeyId],
				secret.Data[AwsSecretAccessKey]),
		),
		"ca_bundle.pem": p.CustomCABundle,
	}
	return nil
}

func (p *AWSProvider) UpdateRegistrySecret(secret, registrySecret *kapi.Secret) error {
	caBundle := p.CustomCABundle
	if p.CustomCABundle == nil {
		// always make sure we have a uniform format of data stored in secret k8s API
		caBundle = []byte{}
	}
	registrySecret.Data = map[string][]byte{
		"access_key":    []byte(secret.Data[AwsAccessKeyId]),
		"secret_key":    []byte(secret.Data[AwsSecretAccessKey]),
		"ca_bundle.pem": caBundle,
	}
	return nil
}

func (p *AWSProvider) UpdateRegistryDeployment(deployment *appsv1.Deployment, name, dirName string) {
	region := p.Region
	if region == "" {
		region = AwsS3DefaultRegion
	}
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	if envVars == nil {
		envVars = []kapi.EnvVar{}
	}
	s3EnvVars := []kapi.EnvVar{
		{
			Name:  "REGISTRY_STORAGE",
			Value: "s3",
		},
		{
			Name: "REGISTRY_STORAGE_S3_ACCESSKEY",
			ValueFrom: &kapi.EnvVarSource{
				SecretKeyRef: &kapi.SecretKeySelector{
					LocalObjectReference: kapi.LocalObjectReference{Name: name},
					Key:                  "access_key",
				},
			},
		},
		{
			Name:  "REGISTRY_STORAGE_S3_BUCKET",
			Value: p.Bucket,
		},
		{
			Name:  "REGISTRY_STORAGE_S3_REGION",
			Value: region,
		},
		{
			Name:  "REGISTRY_STORAGE_S3_REGIONENDPOINT",
			Value: p.S3URL,
		},
		{
			Name:  "REGISTRY_STORAGE_S3_ROOTDIRECTORY",
			Value: "/" + dirName,
		},
		{
			Name: "REGISTRY_STORAGE_S3_SECRETKEY",
			ValueFrom: &kapi.EnvVarSource{
				SecretKeyRef: &kapi.SecretKeySelector{
					LocalObjectReference: kapi.LocalObjectReference{Name: name},
					Key:                  "secret_key",
				},
			},
		},
		{
			Name:  "REGISTRY_STORAGE_S3_SKIPVERIFY",
			Value: strconv.FormatBool(p.Insecure),
		},
	}
	deployment.Spec.Template.Spec.Containers[0].Env = append(envVars, s3EnvVars...)

	if len(p.CustomCABundle) > 0 {
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
			kapi.VolumeMount{
				Name:      "registry-secret",
				ReadOnly:  true,
				MountPath: "/etc/ssl/certs/ca_bundle.pem",
				SubPath:   "ca_bundle.pem",
			})
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, kapi.Volume{
			Name: "registry-secret",
			VolumeSource: kapi.VolumeSource{
				Secret: &kapi.SecretVolumeSource{
					SecretName: name,
				},
			},
		})
	}
}

func (p *AWSProvider) Validate(secret *kapi.Secret) []string {
	fields := []string{}

	if secret != nil {
		keySet := []string{
			AwsAccessKeyId,
			AwsSecretAccessKey,
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
		if !(p.SignatureVersion == "" ||
			p.SignatureVersion == "1" ||
			p.SignatureVersion == "4") {
			fields = append(fields, "SignatureVersion")
		}
		if p.S3URL != "" {
			u, err := url.Parse(p.S3URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				fields = append(fields, "S3URL")
			}
		}
		if p.PublicURL != "" {
			u, err := url.Parse(p.PublicURL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				fields = append(fields, "PublicURL")
			}
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

// Returns `us-east-1` if no region is specified
func (p *AWSProvider) GetRegion() string {
	if p.Region == "" {
		return AwsS3DefaultRegion
	}
	return p.Region
}

// Check the scheme on the configured URL. If a URL is not specified, return
// false
func (p *AWSProvider) GetDisableSSL() bool {
	if p.GetURL() == "" {
		return false
	}
	s3Url, err := url.Parse(p.GetURL())
	if err != nil {
		return false
	}
	if s3Url.Scheme == "" || s3Url.Scheme == "http" {
		return true
	}
	return false
}

// This function returns a boolean determining whether we are talking to an S3
// endpoint that requires path style formatting. Since all S3 APIs support path
// style, the safe approach is to default to path style if the user has
// specified an S3 API URL. This should be updated to perform some smarter
// interpretation of the URL.
func (p *AWSProvider) GetForcePathStyle() bool {
	// If the user has specified a URL, lets assume Path Style for now.
	if p.GetURL() == "" {
		return false
	}
	return true
}

func (p *AWSProvider) Test(secret *kapi.Secret) error {
	var err error

	if secret == nil {
		return nil
	}

	switch p.Role {
	case BackupStorage:
		key, _ := uuid.NewUUID()
		test := S3Test{
			key:            key.String(),
			url:            p.GetURL(),
			region:         p.GetRegion(),
			disableSSL:     p.GetDisableSSL(),
			forcePathStyle: p.GetForcePathStyle(),
			bucket:         p.Bucket,
			secret:         secret,
			customCABundle: p.CustomCABundle,
			insecure:       p.Insecure,
		}
		err = test.Run()
	case VolumeSnapshot:
		// Disable volume snapshot test until
		// https://github.com/konveyor/mig-controller/issues/256 is resolved
		/*test := Ec2Test{
			url:    p.GetURL(),
			region: p.GetRegion(),
			secret: secret,
		}
		err = test.Run()*/
	}

	return err
}

type S3Test struct {
	key            string
	url            string
	region         string
	bucket         string
	disableSSL     bool
	forcePathStyle bool
	customCABundle []byte
	secret         *kapi.Secret
	insecure       bool
}

func (r *S3Test) Run() error {
	ssn, err := r.newSession()
	if err != nil {
		return err
	}
	err = r.upload(ssn)
	if err != nil {
		return err
	}
	defer r.delete(ssn)
	err = r.download(ssn)
	if err != nil {
		return err
	}

	return nil
}

func (r *S3Test) newSession() (*session.Session, error) {
	// copied from net/http
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: r.insecure,
		},
	}

	client := &http.Client{Transport: transport}
	sessionOptions := session.Options{
		Config: aws.Config{
			HTTPClient:       client,
			Region:           &r.region,
			Endpoint:         &r.url,
			DisableSSL:       aws.Bool(r.disableSSL),
			S3ForcePathStyle: aws.Bool(r.forcePathStyle),
			Credentials: credentials.NewStaticCredentials(
				bytes.NewBuffer(r.secret.Data[AwsAccessKeyId]).String(),
				bytes.NewBuffer(r.secret.Data[AwsSecretAccessKey]).String(),
				""),
		},
	}
	if len(r.customCABundle) > 0 {
		sessionOptions.CustomCABundle = bytes.NewReader(r.customCABundle)
	}
	return session.NewSessionWithOptions(sessionOptions)
}

func (r *S3Test) upload(ssn *session.Session) error {
	uploader := s3manager.NewUploader(ssn)
	_, err := uploader.Upload(
		&s3manager.UploadInput{
			Bucket: &r.bucket,
			Body:   bytes.NewReader([]byte{0}),
			Key:    &r.key,
		})

	return err
}

func (r *S3Test) download(ssn *session.Session) error {
	writer := aws.NewWriteAtBuffer([]byte{})
	downloader := s3manager.NewDownloader(ssn)
	_, err := downloader.Download(
		writer,
		&s3.GetObjectInput{
			Bucket: &r.bucket,
			Key:    &r.key,
		})

	return err
}

func (r *S3Test) delete(ssn *session.Session) error {
	_, err := s3.New(ssn).DeleteObject(
		&s3.DeleteObjectInput{
			Bucket: &r.bucket,
			Key:    &r.key,
		})

	return err
}

//
// Ec2 Test
//

type Ec2Test struct {
	url    string
	region string
	secret *kapi.Secret
}

func (r *Ec2Test) Run() error {
	ssn, err := r.newSession()
	if err != nil {
		return err
	}
	// Create volume.
	volumeId, err := r.create(ssn)
	if err != nil {
		return err
	}

	defer r.delete(ssn, volumeId)

	return nil
}

func (r *Ec2Test) newSession() (*session.Session, error) {
	return session.NewSession(&aws.Config{
		Region: &r.region,
		Credentials: credentials.NewStaticCredentials(
			bytes.NewBuffer(r.secret.Data[AwsAccessKeyId]).String(),
			bytes.NewBuffer(r.secret.Data[AwsSecretAccessKey]).String(),
			""),
	})
}

func (r *Ec2Test) create(ssn *session.Session) (*string, error) {
	result, err := ec2.New(ssn).CreateVolume(
		&ec2.CreateVolumeInput{
			AvailabilityZone: aws.String(r.region + "a"),
			VolumeType:       aws.String("gp2"),
			Size:             aws.Int64(1),
		})

	return result.VolumeId, err
}

func (r *Ec2Test) delete(ssn *session.Session, volumeId *string) error {
	_, err := ec2.New(ssn).DeleteVolume(
		&ec2.DeleteVolumeInput{
			VolumeId: volumeId,
		})

	return err
}
