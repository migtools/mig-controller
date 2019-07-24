package cloudprovider

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	"strconv"
)

// Credentials Secret.
const (
	AwsAccessKeyId     = "aws-access-key-id"
	AwsSecretAccessKey = "aws-secret-access-key"
)

// Velero cloud-secret.
var CloudCredentialsTemplete = `
[default]
aws_access_key_id=%s
aws_secret_access_key=%s
`

type AWSProvider struct {
	BaseProvider
	Bucket           string
	Region           string
	S3URL            string
	PublicURL        string
	KMSKeyId         string
	SignatureVersion string
	S3ForcePathStyle bool
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

func (p *AWSProvider) UpdateBSL(bsl *velero.BackupStorageLocation) {
	bsl.Spec.Provider = AWS
	bsl.Spec.StorageType = velero.StorageType{
		ObjectStorage: &velero.ObjectStorageLocation{
			Bucket: p.Bucket,
			Prefix: "velero",
		},
	}
	bsl.Spec.Config = map[string]string{
		"s3ForcePathStyle": strconv.FormatBool(p.S3ForcePathStyle),
		"region":           p.Region,
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
		"region": p.Region,
	}
}

func (p *AWSProvider) UpdateCloudSecret(secret, cloudSecret *kapi.Secret) {
	cloudSecret.Data = map[string][]byte{
		"cloud": []byte(
			fmt.Sprintf(
				CloudCredentialsTemplete,
				secret.Data[AwsAccessKeyId],
				secret.Data[AwsSecretAccessKey]),
		),
	}
}

func (p *AWSProvider) Validate(secret *kapi.Secret) []string {
	fields := []string{}

	if p.Region == "" {
		fields = append(fields, "Region")
	}
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
	case VolumeSnapshot:
	}

	return fields
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
			key:    key.String(),
			url:    p.GetURL(),
			region: p.Region,
			bucket: p.Bucket,
			secret: secret,
		}
		err = test.Run()
	case VolumeSnapshot:
		test := Ec2Test{
			url:    p.GetURL(),
			region: p.Region,
			secret: secret,
		}
		err = test.Run()
	}

	return err
}

type S3Test struct {
	key    string
	url    string
	region string
	bucket string
	secret *kapi.Secret
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
	return session.NewSession(&aws.Config{
		Region: &r.region,
		Credentials: credentials.NewStaticCredentials(
			bytes.NewBuffer(r.secret.Data[AwsAccessKeyId]).String(),
			bytes.NewBuffer(r.secret.Data[AwsSecretAccessKey]).String(),
			""),
	})
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
