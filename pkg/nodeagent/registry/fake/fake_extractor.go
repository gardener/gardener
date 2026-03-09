// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"fmt"
	"io/fs"
	"path"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/spf13/afero"

	"github.com/gardener/gardener/pkg/nodeagent/files"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// FakeRegistryExtractor is a simple implementation of registry.Extractor which can be used to fake the registry extractor in tests.
type FakeRegistryExtractor struct {
	fakeFS          afero.Afero
	sourceDirectory string
	configFile      *configfile.ConfigFile
}

var _ registry.Extractor = &FakeRegistryExtractor{}

// NewExtractor returns a simple implementation of registry.Extractor which can be used to fake the registry extractor in unit tests.
func NewExtractor(fakeFS afero.Afero, sourceDirectory string) *FakeRegistryExtractor {
	return &FakeRegistryExtractor{fakeFS: fakeFS, sourceDirectory: sourceDirectory}
}

// GetConfigFile returns the last configFile passed to CopyFromImage.
func (e *FakeRegistryExtractor) GetConfigFile() *configfile.ConfigFile {
	return e.configFile
}

// CopyFromImage copies a file from a given image reference to the destination file.
func (e *FakeRegistryExtractor) CopyFromImage(_ context.Context, _ string, filePathInImage string, destination string, permissions fs.FileMode, cf *configfile.ConfigFile) error {
	e.configFile = cf
	source := path.Join(e.sourceDirectory, filePathInImage)
	if err := files.Copy(e.fakeFS, source, destination, permissions); err != nil {
		return fmt.Errorf("error copying file %s to %s: %w", source, destination, err)
	}

	return nil
}
