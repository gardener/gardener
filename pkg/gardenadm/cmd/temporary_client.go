// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
)

// NewClientSetFromBootstrapToken returns a Kubernetes client set based on the provided  bootstrap token.
func NewClientSetFromBootstrapToken(controlPlaneAddress string, certificateAuthority []byte, bootstrapToken string, scheme *runtime.Scheme) (kubernetes.Interface, error) {
	return kubernetes.NewWithConfig(kubernetes.WithRESTConfig(&rest.Config{
		Host:            controlPlaneAddress,
		TLSClientConfig: rest.TLSClientConfig{CAData: certificateAuthority},
		BearerToken:     bootstrapToken,
	}), kubernetes.WithClientOptions(client.Options{Scheme: scheme}), kubernetes.WithDisabledCachedClient())
}

// NewClientFromBytes is an alias for kubernetes.NewClientFromBytes.
// Exposed for testing.
var NewClientFromBytes = kubernetes.NewClientFromBytes

// InitializeTemporaryClientSet acquires a short-lived client certificate-based kubeconfig.
func InitializeTemporaryClientSet(ctx context.Context, b *botanist.GardenadmBotanist, bootstrapClientSet kubernetes.Interface) (kubernetes.Interface, error) {
	var (
		certificateSubject = bootstrapCertificateSubject(b)
		cachePath          = CachedBootstrapKubeconfigPath(b.FS, bootstrapClientSet.RESTConfig().Host, certificateSubject.CommonName)
	)

	bootstrapKubeconfig, cached, err := getCachedBootstrapKubeconfig(b.FS, cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed retrieving cached bootstrap kubeconfig: %w", err)
	}

	if !cached {
		bootstrapKubeconfig, err = requestShortLivedBootstrapKubeconfig(ctx, b, bootstrapClientSet, certificateSubject)
		if err != nil {
			return nil, fmt.Errorf("failed to request short-lived bootstrap kubeconfig via CertificateSigningRequest API: %w", err)
		}

		if err := writeCachedBootstrapKubeconfig(b.FS, cachePath, bootstrapKubeconfig); err != nil {
			return nil, fmt.Errorf("failed writing the retrieved bootstrap kubeconfig to a temporary file: %w", err)
		}
	}

	// The kubeconfig bytes come from a cache file that an unprivileged local user might be able to plant, so we must
	// not hand them to NewClientFromBytes unchecked - it skips Gardener's kubeconfig field validation and would honor,
	// e.g., an exec credential plugin (local code execution) or a foreign API server.
	config, err := clientcmd.Load(bootstrapKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed loading bootstrap kubeconfig: %w", err)
	}
	if err := kubernetes.ValidateConfig(*config); err != nil {
		return nil, fmt.Errorf("bootstrap kubeconfig failed validation: %w", err)
	}

	return NewClientFromBytes(
		bootstrapKubeconfig,
		kubernetes.WithClientOptions(client.Options{Scheme: bootstrapClientSet.Client().Scheme()}),
		kubernetes.WithDisabledCachedClient(),
	)
}

// writeCachedBootstrapKubeconfig writes the bootstrap kubeconfig to its cache file. The parent directory is created
// with mode 0700 and the file with O_EXCL, so that an unprivileged local user sharing the temporary directory can
// neither plant a file at the predictable path nor have a planted file be trusted on the next read.
func writeCachedBootstrapKubeconfig(fs afero.Afero, path string, bootstrapKubeconfig []byte) error {
	if err := fs.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed creating cache directory: %w", err)
	}

	// On a cache miss any pre-existing file is either expired or foreign, hence we remove it before the O_EXCL create.
	if err := fs.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed removing stale cache file: %w", err)
	}

	file, err := fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(bootstrapKubeconfig); err != nil {
		return err
	}

	return nil
}

// CachedBootstrapKubeconfigPath returns the path of the file caching the short-lived bootstrap kubeconfig. It lives in
// an owner-only `gardenadm` directory (see writeCachedBootstrapKubeconfig) so that unprivileged local users sharing the
// temporary directory cannot plant a file. The file name is specific to the target API server and the requested user,
// so that executing gardenadm against different clusters (e.g., when running `gardenadm connect` outside the control
// plane machines) never reuses foreign credentials.
func CachedBootstrapKubeconfigPath(fs afero.Afero, host, commonName string) string {
	return filepath.Join(fs.GetTempDir(""), "gardenadm", "bootstrap-kubeconfig-"+utils.ComputeSHA256Hex([]byte(host + "\n" + commonName))[:16])
}

const bootstrapKubeconfigValidity = 10 * time.Minute

func getCachedBootstrapKubeconfig(fs afero.Afero, path string) ([]byte, bool, error) {
	fileInfo, err := fs.Stat(path)
	if err != nil || time.Since(fileInfo.ModTime()) > bootstrapKubeconfigValidity-2*time.Minute {
		// We deliberately ignore the error here - this is just a best-effort attempt to cache the bootstrap kubeconfig.
		// If the file doesn't exist, or we cannot read/find it for whatever reason, we just consider it as a cache
		// miss.
		// Otherwise, if the last modifications time of the file is older than the validity of the bootstrap kubeconfig,
		// we consider it as expired and thus a cache miss.
		return nil, false, nil //nolint:nilerr
	}

	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed reading the cached bootstrap kubeconfig: %w", err)
	}

	return data, true, nil
}

func bootstrapCertificateSubject(b *botanist.GardenadmBotanist) *pkix.Name {
	commonNameSuffix := b.HostName
	if b.Shoot != nil && b.Shoot.GetInfo() != nil {
		commonNameSuffix = b.Shoot.GetInfo().Namespace + ":" + b.Shoot.GetInfo().Name
	}

	return &pkix.Name{
		Organization: []string{v1beta1constants.ShootsGroup},
		CommonName:   v1beta1constants.GardenadmUserNamePrefix + commonNameSuffix,
	}
}

func requestShortLivedBootstrapKubeconfig(ctx context.Context, b *botanist.GardenadmBotanist, bootstrapClientSet kubernetes.Interface, certificateSubject *pkix.Name) ([]byte, error) {
	certData, privateKeyData, _, err := certificatesigningrequest.RequestCertificate(ctx, b.Logger, bootstrapClientSet.Kubernetes(), certificateSubject, []string{}, []net.IP{}, &metav1.Duration{Duration: bootstrapKubeconfigValidity}, "gardenadm-csr-")
	if err != nil {
		return nil, fmt.Errorf("unable to bootstrap the kubeconfig: %w", err)
	}

	return gardenletbootstraputil.CreateKubeconfigWithClientCertificate(bootstrapClientSet.RESTConfig(), privateKeyData, certData)
}
