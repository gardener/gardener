// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Generator renders an OperatingSystemConfig into a
// representation suitable for an specific OS
// also returns the os specific command for applying this configuration
type Generator interface {
	Generate(*OperatingSystemConfig) (osconfig []byte, command *string, err error)
}

// File is a file to be stored during the cloud init script.
type File struct {
	Path        string
	Content     []byte
	Permissions *int32
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
