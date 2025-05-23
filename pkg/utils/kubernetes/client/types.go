// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Finalizer checks and removes the finalizers of given resource.
type Finalizer interface {
	// Finalize removes the resource finalizers (so it can be garbage collected).
	Finalize(ctx context.Context, c client.Client, obj client.Object) error

	// HasFinalizers checks whether the given resource has finalizers.
	HasFinalizers(obj client.Object) (bool, error)
}

// Cleaner is capable of deleting and finalizing resources.
type Cleaner interface {
	// Clean cleans the given resource(s). It first tries to delete them. If they are 'hanging'
	// in deletion state and `FinalizeGracePeriodSeconds` is specified, then they are finalized
	// once the `deletionTimestamp` is beyond that amount in the past.
	Clean(ctx context.Context, c client.Client, obj runtime.Object, opts ...CleanOption) error
}

// GoneEnsurer ensures that resource(s) are gone.
type GoneEnsurer interface {
	// EnsureGone ensures that the given resource is gone. If the resource is not gone, it will throw
	// a NewObjectsRemaining error.
	EnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...client.ListOption) error
}

// GoneEnsurerFunc is a function that implements GoneEnsurer.
type GoneEnsurerFunc func(ctx context.Context, c client.Client, obj runtime.Object, opts ...client.ListOption) error

// CleanOps are ops to clean.
type CleanOps interface {
	// CleanAndEnsureGone cleans the resource(s) and ensures that it/they are gone afterwards.
	CleanAndEnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...CleanOption) error
}
