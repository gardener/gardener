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

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
)

const (
	mediaTypeHelm = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"

	localRegistry        = "localhost:5001"
	inKubernetesRegistry = "garden.local.gardener.cloud:5001"
)

// Interface represents an OCI compatible regisry.
type Interface interface {
	Pull(ctx context.Context, oci *gardencorev1.OCIRepository) ([]byte, error)
}

// HelmRegistry can pull OCI Helm Charts.
type HelmRegistry struct {
	cache cacher
}

// NewHelmRegistry creates a new HelmRegistry.
func NewHelmRegistry() (*HelmRegistry, error) {
	return &HelmRegistry{
		cache: defaultCache,
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
        if strings.Contains(ref, "garden.local.gardener.cloud:5001") {
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
