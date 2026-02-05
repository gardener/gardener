// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	mediaTypeHelm = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"

	localRegistryPattern = "registry.local.gardener.cloud:"
)

type secretNamespace struct{}

var (
	// ContextKeySecretNamespace is the key to use to pass the secret namespace in the context.
	ContextKeySecretNamespace = secretNamespace{}
)

// Interface represents an OCI compatible registry.
type Interface interface {
	// Pull from the repository and return the Helm chart.
	// The context can be used to pass the secret namespace with the key ContextKeySecretNamespace.
	Pull(ctx context.Context, oci *gardencorev1.OCIRepository) ([]byte, error)
}

// HelmRegistry can pull OCI Helm Charts.
type HelmRegistry struct {
	cache  cacher
	client client.Client
}

// NewHelmRegistry creates a new HelmRegistry.
// The client is used to get pull secrets if needed.
func NewHelmRegistry(c client.Client) *HelmRegistry {
	return &HelmRegistry{
		cache:  defaultCache,
		client: c,
	}
}

// Pull from the repository and return the compressed archive.
func (r *HelmRegistry) Pull(ctx context.Context, oci *gardencorev1.OCIRepository) ([]byte, error) {
	ref, err := buildRef(oci)
	if err != nil {
		return nil, err
	}
	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
	}

	secretNamespace := v1beta1constants.GardenNamespace
	if oci.CABundleSecretRef != nil || oci.PullSecretRef != nil {
		if v := ctx.Value(ContextKeySecretNamespace); v != nil {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("secret namespace must be a string, got %T instead", v)
			}
			secretNamespace = s
		}
	}

	// Configure custom transport with CA bundle if provided
	if oci.CABundleSecretRef != nil {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: secretNamespace, Name: oci.CABundleSecretRef.Name}}
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			return nil, fmt.Errorf("failed to get CA bundle secret %s: %w", client.ObjectKeyFromObject(secret), err)
		}
		caBundle, ok := secret.Data[secretsutils.DataKeyCertificateBundle]
		if !ok {
			return nil, fmt.Errorf("CA bundle secret %s is missing the data key %s", client.ObjectKeyFromObject(secret), secretsutils.DataKeyCertificateBundle)
		}
		if len(caBundle) == 0 {
			return nil, fmt.Errorf("CA bundle secret %s has empty data for key %s", client.ObjectKeyFromObject(secret), secretsutils.DataKeyCertificateBundle)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caBundle) {
			return nil, errors.New("failed to append CA certificates from bundle")
		}

		transport := remote.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		}
		remoteOpts = append(remoteOpts, remote.WithTransport(transport))
	}

	if oci.PullSecretRef != nil {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: secretNamespace, Name: oci.PullSecretRef.Name}}
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			return nil, fmt.Errorf("failed to get pull secret %s: %w", client.ObjectKeyFromObject(secret), err)
		}
		if secret.Data[corev1.DockerConfigJsonKey] == nil {
			return nil, fmt.Errorf("pull secret %s is missing the data key %s", client.ObjectKeyFromObject(secret), corev1.DockerConfigJsonKey)
		}
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(&keychain{pullSecret: string(secret.Data[corev1.DockerConfigJsonKey])}))
	}

	key, err := cacheKeyFromRef(ref, remoteOpts...)
	if err != nil {
		return nil, err
	}
	if key != "" {
		if blob, found := r.cache.Get(key); found {
			return blob, nil
		}
	}

	img, err := remote.Image(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull artifact %s: %w", ref, err)
	}
	blob, err := extractHelmLayer(img)
	if err != nil {
		return nil, err
	}

	// construct cache key based on digest of the pulled artifact
	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}
	key = ref.Context().Digest(digest.String()).Name()
	r.cache.Set(key, blob)

	return blob, nil
}

func buildRef(oci *gardencorev1.OCIRepository) (name.Reference, error) {
	ref := oci.GetURL()

	opts := []name.Option{
		name.StrictValidation,
	}

	// in the local setup we don't want to use TLS
	if strings.Contains(ref, localRegistryPattern) {
		opts = append(opts, name.Insecure)
	}

	return name.ParseReference(ref, opts...)
}

// cacheKeyFromRef returns "repo@sha256:digest". If the ref is not a digest, the remote repository is queried to
// retrieve the digest pointed to by the ref.
func cacheKeyFromRef(ref name.Reference, opts ...remote.Option) (string, error) {
	if ref, ok := ref.(name.Digest); ok {
		return ref.Name(), nil
	}

	var digest gcrv1.Hash
	desc, hErr := remote.Head(ref, opts...)
	if hErr == nil {
		digest = desc.Digest
	} else {
		rd, gErr := remote.Get(ref, opts...)
		if gErr != nil {
			return "", fmt.Errorf("failed get manifest from remote trying to determine digest: %w", errors.Join(gErr, hErr))
		}
		digest = rd.Digest
	}
	return ref.Context().Digest(digest.String()).Name(), nil
}

func extractHelmLayer(image gcrv1.Image) ([]byte, error) {
	layers, err := image.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to parse layers: %w", err)
	}

	if len(layers) < 1 {
		return nil, fmt.Errorf("no layers found")
	}

	var layer gcrv1.Layer
	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			return nil, err
		}
		if string(mt) == mediaTypeHelm {
			layer = l
			break
		}
	}
	if layer == nil {
		return nil, fmt.Errorf("no helm layer found in artifact")
	}
	blob, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to extract layer from artifact: %w", err)
	}
	raw, err := io.ReadAll(blob)
	if err != nil {
		return nil, fmt.Errorf("failed to read content of helm layer: %w", err)
	}
	return raw, nil
}
