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

package webhook

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// Mutator validates and if needed mutates objects.
type Mutator interface {
	// Mutate validates and if needed mutates the given object.
	// "old" is optional and it must always be checked for nil.
	Mutate(ctx context.Context, new, old client.Object) error
}

// MutatorWithShootClient validates and if needed mutates objects. It needs the shoot client.
type MutatorWithShootClient interface {
	// Mutate validates and if needed mutates the given object.
	// "old" is optional and it must always be checked for nil.
	Mutate(ctx context.Context, new, old client.Object, shootClient client.Client) error
}
type mutatorWrapper struct {
	Mutator
}

// InjectFunc calls the inject.Func on the handler mutators.
func (d *mutatorWrapper) InjectFunc(f inject.Func) error {
	if err := f(d.Mutator); err != nil {
		return fmt.Errorf("could not inject into the mutator: %w", err)
	}
	return nil
}

func hybridMutator(mut Mutator) Mutator {
	return &mutatorWrapper{mut}
}

// MutateFunc is a func to be used directly as an implementation for Mutator
type MutateFunc func(ctx context.Context, new, old client.Object) error

// Mutate validates and if needed mutates the given object.
func (mf MutateFunc) Mutate(ctx context.Context, new, old client.Object) error {
	return mf(ctx, new, old)
}
