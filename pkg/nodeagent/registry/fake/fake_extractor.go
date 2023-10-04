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
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// Extraction is a fake implementation of Extractor for unit tests.
type Extraction struct {
	Image      string
	PathSuffix string
	Dest       string
}

type fakeRegistryExtractor struct {
	extractions []Extraction
}

var _ registry.Extractor = &fakeRegistryExtractor{}

// New returns a simple implementation of registry.Extractor which can be used to fake the registry extractor in unit
// tests.
func New(extractions ...Extraction) *fakeRegistryExtractor {
	return &fakeRegistryExtractor{extractions: extractions}
}

func (f *fakeRegistryExtractor) ExtractFromLayer(image, pathSuffix, dest string) error {
	f.extractions = append(f.extractions, Extraction{
		Image:      image,
		PathSuffix: pathSuffix,
		Dest:       dest,
	})
	return nil
}
