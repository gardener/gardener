// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package manager

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func (m *manager) Get(name string, opts ...GetOption) (*corev1.Secret, error) {
	options := &GetOptions{}
	options.ApplyOptions(opts)

	secrets, found := m.getFromStore(name)
	if !found {
		return nil, fmt.Errorf("secrets for name %q not found in internal store", name)
	}

	class := bundle
	if options.Class != nil {
		class = *options.Class
	} else if secrets.bundle == nil {
		class = current // fall back to current secret if there is no bundle secret and if class was not explicitly set
	}

	switch class {
	case current:
		return secrets.current.obj, nil
	case old:
		if secrets.old == nil {
			return nil, fmt.Errorf("there is no old object for the secret with name %q", name)
		}
		return secrets.old.obj, nil
	default:
		if secrets.bundle == nil {
			return nil, fmt.Errorf("there is no bundle object for the secret with name %q", name)
		}
		return secrets.bundle.obj, nil
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
