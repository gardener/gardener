// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"fmt"
	"io/fs"
	"path"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/nodeagent/files"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// FakeRegistryExtractor is a simple implementation of registry.Extractor which can be used to fake the registry extractor in tests.
type FakeRegistryExtractor struct {
	fakeFS          afero.Afero
	sourceDirectory string
}

var _ registry.Extractor = &FakeRegistryExtractor{}

// NewExtractor returns a simple implementation of registry.Extractor which can be used to fake the registry extractor in unit tests.
func NewExtractor(fakeFS afero.Afero, sourceDirectory string) *FakeRegistryExtractor {
	return &FakeRegistryExtractor{fakeFS: fakeFS, sourceDirectory: sourceDirectory}
}

// CopyFromImage copies a file from a given image reference to the destination file.
func (e *FakeRegistryExtractor) CopyFromImage(_ context.Context, _ string, _ *corev1.Secret, filePathInImage string, destination string, permissions fs.FileMode) error {
	source := path.Join(e.sourceDirectory, filePathInImage)
	if err := files.Copy(e.fakeFS, source, destination, permissions); err != nil {
		return fmt.Errorf("error copying file %s to %s: %w", source, destination, err)
	}

	return nil
}
