package cloudprovider

import (
	"github.com/onsi/gomega"
	"testing"
)

func TestValidateRejectsInternalSvcEndpoints(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name         string
		s3URL        string
		publicURL    string
		expectS3Svc  bool
		expectPubSvc bool
	}{
		{
			name:        "internal .svc S3 endpoint",
			s3URL:       "https://rook-ceph-rgw-ocs-storagecluster.openshift-storage.svc",
			expectS3Svc: true,
		},
		{
			name:         "internal .svc public endpoint",
			publicURL:    "http://rgw.openshift-storage.svc",
			expectPubSvc: true,
		},
		{
			name:        "internal .svc.cluster.local S3 endpoint",
			s3URL:       "https://rook-ceph-rgw-ocs-storagecluster.openshift-storage.svc.cluster.local",
			expectS3Svc: true,
		},
		{
			name:         "internal .svc.cluster.local public endpoint",
			publicURL:    "http://rgw.openshift-storage.svc.cluster.local",
			expectPubSvc: true,
		},
		{
			name:  "external S3 endpoint is allowed",
			s3URL: "https://s3.example.com",
		},
		{
			name:      "external route is allowed",
			publicURL: "https://rgw-openshift-storage.apps.cluster.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := AWSProvider{
				BaseProvider: BaseProvider{Role: BackupStorage},
				Bucket:       "test-bucket",
				S3URL:        tt.s3URL,
				PublicURL:    tt.publicURL,
			}
			fields := p.Validate(nil)
			hasS3Svc := false
			hasPubSvc := false
			for _, f := range fields {
				if f == "S3URL-InternalEndpoint" {
					hasS3Svc = true
				}
				if f == "PublicURL-InternalEndpoint" {
					hasPubSvc = true
				}
			}
			g.Expect(hasS3Svc).To(gomega.Equal(tt.expectS3Svc), "S3URL-InternalEndpoint")
			g.Expect(hasPubSvc).To(gomega.Equal(tt.expectPubSvc), "PublicURL-InternalEndpoint")
		})
	}
}

func TestGetDisableSSL(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	p := AWSProvider{S3URL: ""}
	p.GetDisableSSL()
	g.Expect(p.GetDisableSSL()).To(gomega.BeFalse())

	p.S3URL = "https://example.com"
	p.GetDisableSSL()
	g.Expect(p.GetDisableSSL()).To(gomega.BeFalse())

	p.S3URL = "example.com"
	p.GetDisableSSL()
	g.Expect(p.GetDisableSSL()).To(gomega.BeTrue())

	p.S3URL = "http://example.com"
	p.GetDisableSSL()
	g.Expect(p.GetDisableSSL()).To(gomega.BeTrue())
}
