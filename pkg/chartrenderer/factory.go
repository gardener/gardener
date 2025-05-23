// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package chartrenderer

import (
	"k8s.io/client-go/rest"
)

// Factory is a factory that is able to produce Interface.
type Factory interface {
	NewForConfig(config *rest.Config) (Interface, error)
}

// FactoryFunc implements the Factory interface.
type FactoryFunc func(config *rest.Config) (Interface, error)

// NewForConfig implements Factory.
func (f FactoryFunc) NewForConfig(config *rest.Config) (Interface, error) {
	return f(config)
}

// DefaultFactory returns the default Factory.
func DefaultFactory() Factory {
	return FactoryFunc(NewForConfig)
}
