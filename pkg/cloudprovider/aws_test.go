package cloudprovider

import (
	"github.com/onsi/gomega"
	"testing"
)

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
