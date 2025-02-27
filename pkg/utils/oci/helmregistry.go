// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	mediaTypeHelm = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"

	inKubernetesRegistry = "garden.local.gardener.cloud:5001"
)

type pullSecretNamespace struct{}

// ContextKeyPullSecretNamespace is the key to use to pass the pull secret namespace in the context.
var ContextKeyPullSecretNamespace = pullSecretNamespace{}

// Interface represents an OCI compatible registry.
type Interface interface {
	// Pull from the repository and return the Helm chart.
	// The context can be used to pass the pull secret namespace with the key ContextKeyPullSecretNamespace.
	Pull(ctx context.Context, oci *gardencorev1.OCIRepository) ([]byte, error)
}

// HelmRegistry can pull OCI Helm Charts.
type HelmRegistry struct {
	cache  cacher
	client client.Client
}

// NewHelmRegistry creates a new HelmRegistry.
// The client is used to get pull secrets if needed.
func NewHelmRegistry(c client.Client) (*HelmRegistry, error) {
	return &HelmRegistry{
		cache:  defaultCache,
		client: c,
	}, nil
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

	if oci.PullSecretRef != nil {
		namespace := v1beta1constants.GardenNamespace
		if v := ctx.Value(ContextKeyPullSecretNamespace); v != nil {
			s, ok := v.(string)
			if !ok {
				return nil, errors.New("pull secret namespace must be a string")
			}
			namespace = s
		}
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: oci.PullSecretRef.Name}}
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
	if strings.Contains(ref, inKubernetesRegistry) {
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
		digest = rd.Descriptor.Digest
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
