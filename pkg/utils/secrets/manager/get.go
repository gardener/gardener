// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	corev1 "k8s.io/api/core/v1"
)

func (m *manager) Get(name string, opts ...GetOption) (*corev1.Secret, bool) {
	options := &GetOptions{}
	options.ApplyOptions(opts)

	secrets, found := m.getFromStore(name)
	if !found {
		return nil, false
	}

	class := bundle
	if options.Class != nil {
		class = *options.Class
	} else if secrets.bundle == nil {
		class = current // fall back to current secret if there is no bundle secret and if class was not explicitly set
	}

	switch class {
	case current:
		return secrets.current.obj, true
	case old:
		if secrets.old == nil {
			return nil, false
		}
		return secrets.old.obj, true
	default:
		if secrets.bundle == nil {
			return nil, false
		}
		return secrets.bundle.obj, true
	}
}

// GetOption is some configuration that modifies options for a Get request.
type GetOption interface {
	// ApplyToOptions applies this configuration to the given options.
	ApplyToOptions(*GetOptions)
}

// GetOptions are options for Get calls.
type GetOptions struct {
	// Class specifies whether which secret should be returned. By default, the bundle secret is returned. If there is
	// no bundle secret then it falls back to the current secret.
	Class *secretClass
}

// ApplyOptions applies the given update options on these options, and then returns itself (for convenient chaining).
func (o *GetOptions) ApplyOptions(opts []GetOption) *GetOptions {
	for _, opt := range opts {
		opt.ApplyToOptions(o)
	}
	return o
}

var (
	// Current sets the Class field to 'current' in the GetOptions.
	Current = classOption{class: current}
	// Old sets the Class field to 'old' in the GetOptions.
	Old = classOption{class: old}
	// Bundle sets the Class field to 'bundle' in the GetOptions.
	Bundle = classOption{class: bundle}
)

type classOption struct {
	class secretClass
}

func (c classOption) ApplyToOptions(options *GetOptions) {
	options.Class = &c.class
}
