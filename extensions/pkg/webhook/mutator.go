// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Mutator validates and if needed mutates objects.
type Mutator interface {
	// Mutate validates and if needed mutates the given object.
	// "old" is optional and it must always be checked for nil.
	Mutate(ctx context.Context, new, old runtime.Object) error
}

// MutatorWithShootClient validates and if needed mutates objects. It needs the shoot client.
type MutatorWithShootClient interface {
	// Mutate validates and if needed mutates the given object.
	// "old" is optional and it must always be checked for nil.
	Mutate(ctx context.Context, new, old runtime.Object, shootClient client.Client) error
}

// MutateFunc is a func to be used directly as an implementation for Mutator
type MutateFunc func(ctx context.Context, new, old runtime.Object) error

// Mutate validates and if needed mutates the given object.
func (mf MutateFunc) Mutate(ctx context.Context, new, old runtime.Object) error {
	return mf(ctx, new, old)
}
