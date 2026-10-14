package directimagestreammigration

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/types"
	liberr "github.com/konveyor/controller/pkg/error"
	v1alpha1 "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/openshift-velero-plugin/velero-plugins/imagecopy"
)

func (t *Task) migrateInternalImages() error {
	imageStream, err := t.Owner.GetImageStream(t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	srcCluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	destCluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}

	srcInternalRegistry, err := srcCluster.GetInternalRegistryPath(t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcInternalRegistry == "" {
		return liberr.Wrap(errors.New("Source cluster internal registry path not found"))
	}

	srcRegistry, err := srcCluster.GetRegistryPath(t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcRegistry == "" {
		return liberr.Wrap(errors.New("Source cluster registry path not found"))
	}

	destRegistry, err := destCluster.GetRegistryPath(t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if destRegistry == "" {
		return liberr.Wrap(errors.New("Source cluster registry path not found"))
	}

	destNamespace := t.Owner.GetDestinationNamespace()
	if destNamespace == "" {
		return liberr.Wrap(errors.New("Destination namespace not found"))
	}

	srcClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	sourceCtx, srcTempDir, err := internalRegistrySystemContext(srcClient, srcCluster)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcTempDir != "" {
		defer os.RemoveAll(srcTempDir)
	}

	destClient, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	destinationCtx, destTempDir, err := internalRegistrySystemContext(destClient, destCluster)
	if err != nil {
		return liberr.Wrap(err)
	}
	if destTempDir != "" {
		defer os.RemoveAll(destTempDir)
	}

	return imagecopy.CopyLocalImageStreamImages(*imageStream,
		srcInternalRegistry,
		srcRegistry,
		destRegistry,
		destNamespace,
		&copy.Options{
			SourceCtx:      sourceCtx,
			DestinationCtx: destinationCtx,
		},
		t.Log,
		false)
}

func internalRegistrySystemContext(c compat.Client, cluster *v1alpha1.MigCluster) (*types.SystemContext, string, error) {
	config := c.RestConfig()
	if config.BearerToken == "" {
		return nil, "", errors.New("BearerToken not found, can't authenticate with registry")
	}

	// Enable TLS verification by default, only disable if explicitly set
	var insecureSkipTLS types.OptionalBool
	if cluster.Spec.Insecure {
		insecureSkipTLS = types.OptionalBoolTrue
	} else {
		insecureSkipTLS = types.OptionalBoolFalse
	}

	ctx := &types.SystemContext{
		DockerDaemonInsecureSkipTLSVerify: cluster.Spec.Insecure,
		DockerInsecureSkipTLSVerify:       insecureSkipTLS,
		DockerDisableDestSchema1MIMETypes: true,
		DockerAuthConfig: &types.DockerAuthConfig{
			Username: "ignored",
			Password: config.BearerToken,
		},
	}

	var tempDir string
	// Configure custom CA bundle if TLS is enabled and CABundle is provided
	if !cluster.Spec.Insecure && len(cluster.Spec.CABundle) > 0 {
		// Create temporary directory for CA certificate
		var err error
		tempDir, err = os.MkdirTemp("", "registry-ca-")
		if err != nil {
			return nil, "", liberr.Wrap(err)
		}

		// Write CA bundle to ca.crt in the temp directory
		caFile := filepath.Join(tempDir, "ca.crt")
		if err := os.WriteFile(caFile, cluster.Spec.CABundle, 0600); err != nil {
			os.RemoveAll(tempDir)
			return nil, "", liberr.Wrap(err)
		}

		// Point registry client to the CA certificate directory
		ctx.DockerCertPath = tempDir
	}

	return ctx, tempDir, nil
}
