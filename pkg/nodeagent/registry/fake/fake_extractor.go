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
	"path"

	"github.com/spf13/afero"

	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

type fakeRegistryExtractor struct {
	aferoFS         afero.Afero
	sourceDirectory string
}

var _ registry.Extractor = &fakeRegistryExtractor{}

// NewExtractor returns a simple implementation of registry.Extractor which can be used to fake the registry extractor in unit tests.
func NewExtractor(aferoFS afero.Afero, sourceDirectory string) registry.Extractor {
	return &fakeRegistryExtractor{aferoFS: aferoFS, sourceDirectory: sourceDirectory}
}

// CopyFromImage copies files from a given image reference to the destination folder.
func (e *fakeRegistryExtractor) CopyFromImage(_ context.Context, _ string, files []string, destination string) error {
	for _, file := range files {
		sourceFile := path.Join(e.sourceDirectory, file)
		destinationFile := path.Join(destination, path.Base(file))
		if err := registry.CopyFile(e.aferoFS, sourceFile, destinationFile); err != nil {
			return fmt.Errorf("error copying file %s to %s: %w", sourceFile, destinationFile, err)
		}
	}

	return nil
}
