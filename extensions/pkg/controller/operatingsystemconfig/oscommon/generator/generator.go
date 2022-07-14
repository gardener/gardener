// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/go-logr/logr"
)

// Generator renders an OperatingSystemConfig into a
// representation suitable for an specific OS
// also returns the os specific command for applying this configuration
type Generator interface {
	Generate(logr.Logger, *OperatingSystemConfig) (osconfig []byte, command *string, err error)
}

// File is a file to be stored during the cloud init script.
type File struct {
	Path              string
	Content           []byte
	Permissions       *int32
	TransmitUnencoded *bool
}

// Unit is a unit to be created during the cloud init script.
type Unit struct {
	Name    string
	Content []byte
	DropIns []*DropIn
}

// DropIn is a drop in of a Unit.
type DropIn struct {
	Name    string
	Content []byte
}

// OperatingSystemConfig is the data required to create a cloud init script.
type OperatingSystemConfig struct {
	Object    *extensionsv1alpha1.OperatingSystemConfig
	CRI       *extensionsv1alpha1.CRIConfig
	Files     []*File
	Units     []*Unit
	Bootstrap bool
	Path      *string
}
