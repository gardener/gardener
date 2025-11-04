// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
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
	bootstrapKubeconfig, cached, err := getCachedBootstrapKubeconfig(b)
	if err != nil {
		return nil, fmt.Errorf("failed retrieving cached bootstrap kubeconfig: %w", err)
	}

	if !cached {
		bootstrapKubeconfig, err = requestShortLivedBootstrapKubeconfig(ctx, b, bootstrapClientSet)
		if err != nil {
			return nil, fmt.Errorf("failed to request short-lived bootstrap kubeconfig via CertificateSigningRequest API: %w", err)
		}

		if err := b.FS.WriteFile(cachedBootstrapKubeconfigPath(b.FS), bootstrapKubeconfig, 0600); err != nil {
			return nil, fmt.Errorf("failed writing the retrieved bootstrap kubeconfig to a temporary file: %w", err)
		}
	}

	return NewClientFromBytes(
		bootstrapKubeconfig,
		kubernetes.WithClientOptions(client.Options{Scheme: bootstrapClientSet.Client().Scheme()}),
		kubernetes.WithDisabledCachedClient(),
	)
}

func cachedBootstrapKubeconfigPath(fs afero.Afero) string {
	return filepath.Join(fs.GetTempDir(""), "gardenadm-bootstrap-kubeconfig")
}

const bootstrapKubeconfigValidity = 10 * time.Minute

func getCachedBootstrapKubeconfig(b *botanist.GardenadmBotanist) ([]byte, bool, error) {
	fileInfo, err := b.FS.Stat(cachedBootstrapKubeconfigPath(b.FS))
	if err != nil || time.Since(fileInfo.ModTime()) > bootstrapKubeconfigValidity-2*time.Minute {
		// We deliberately ignore the error here - this is just a best-effort attempt to cache the bootstrap kubeconfig.
		// If the file doesn't exist, or we cannot read/find it for whatever reason, we just consider it as a cache
		// miss.
		// Otherwise, if the last modifications time of the file is older than the validity of the bootstrap kubeconfig,
		// we consider it as expired and thus a cache miss.
		return nil, false, nil //nolint:nilerr
	}

	data, err := b.FS.ReadFile(cachedBootstrapKubeconfigPath(b.FS))
	if err != nil {
		return nil, false, fmt.Errorf("failed reading the cached bootstrap kubeconfig: %w", err)
	}

	return data, true, nil
}

func requestShortLivedBootstrapKubeconfig(ctx context.Context, b *botanist.GardenadmBotanist, bootstrapClientSet kubernetes.Interface) ([]byte, error) {
	commonNameSuffix := b.HostName
	if b.Shoot != nil && b.Shoot.GetInfo() != nil {
		commonNameSuffix = b.Shoot.GetInfo().Namespace + ":" + b.Shoot.GetInfo().Name
	}

	certificateSubject := &pkix.Name{
		Organization: []string{v1beta1constants.ShootsGroup},
		CommonName:   v1beta1constants.GardenadmUserNamePrefix + commonNameSuffix,
	}

	certData, privateKeyData, _, err := certificatesigningrequest.RequestCertificate(ctx, b.Logger, bootstrapClientSet.Kubernetes(), certificateSubject, []string{}, []net.IP{}, &metav1.Duration{Duration: bootstrapKubeconfigValidity}, "gardenadm-csr-")
	if err != nil {
		return nil, fmt.Errorf("unable to bootstrap the kubeconfig: %w", err)
	}

	return gardenletbootstraputil.CreateKubeconfigWithClientCertificate(bootstrapClientSet.RESTConfig(), privateKeyData, certData)
}
