// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fake

import (
	"context"
	"fmt"
	"io/fs"
	"path"

	"github.com/spf13/afero"

	"github.com/gardener/gardener/pkg/nodeagent/files"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

type fakeRegistryExtractor struct {
	fakeFS          afero.Afero
	sourceDirectory string
}

var _ registry.Extractor = &fakeRegistryExtractor{}

// NewExtractor returns a simple implementation of registry.Extractor which can be used to fake the registry extractor in unit tests.
func NewExtractor(fakeFS afero.Afero, sourceDirectory string) registry.Extractor {
	return &fakeRegistryExtractor{fakeFS: fakeFS, sourceDirectory: sourceDirectory}
}

// CopyFromImage copies a file from a given image reference to the destination file.
func (e *fakeRegistryExtractor) CopyFromImage(_ context.Context, _ string, filePathInImage string, destination string, permissions fs.FileMode) error {
	source := path.Join(e.sourceDirectory, filePathInImage)
	if err := files.Copy(e.fakeFS, source, destination, permissions); err != nil {
		return fmt.Errorf("error copying file %s to %s: %w", source, destination, err)
	}

	return nil
}
