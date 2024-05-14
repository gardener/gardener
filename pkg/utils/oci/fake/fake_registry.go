// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"fmt"
	"sync"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	"github.com/gardener/gardener/pkg/utils/oci"
)

var _ oci.Interface = &Registry{}

// Registry implements oci.Interface and returns the artifacts previously added via `.SetArtifact()`.
type Registry struct {
	mu        sync.Mutex
	artifacts map[string][]byte
}

// NewRegistry returns a new registry
func NewRegistry() *Registry {
	return &Registry{
		artifacts: make(map[string][]byte),
	}
}

// Pull implements registry.Interface
func (r *Registry) Pull(_ context.Context, oci *gardencorev1.OCIRepository) ([]byte, error) {
	data, ok := r.artifacts[artifactKey(oci)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return data, nil
}

// AddArtifact adds an artifact to the fake registry.
func (r *Registry) AddArtifact(oci *gardencorev1.OCIRepository, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.artifacts[artifactKey(oci)] = data
}

func artifactKey(oci *gardencorev1.OCIRepository) string {
	if oci.Ref != "" {
		return oci.Ref
	}
	return fmt.Sprintf("%s:%s@%s", oci.Repository, oci.Tag, oci.Digest)
}
