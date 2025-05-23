// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// AddFinalizers ensures that the given finalizer is present in the given object and optimistic locking. If it is not
// set, it adds it and issues a patch.
// Note that this is done with a regular merge-patch since strategic merge-patches do not work with custom resources,
// see https://github.com/kubernetes/kubernetes/issues/105146.
func AddFinalizers(ctx context.Context, writer client.Writer, obj client.Object, finalizers ...string) error {
	return patchFinalizers(ctx, writer, obj, mergeFromWithOptimisticLock, controllerutil.AddFinalizer, finalizers...)
}

// RemoveFinalizers ensures that the given finalizer is not present anymore in the given object and optimistic locking.
// If it is set, it removes it and issues a patch.
// Note that this is done with a regular merge-patch since strategic merge-patches do not work with custom resources,
// see https://github.com/kubernetes/kubernetes/issues/105146.
func RemoveFinalizers(ctx context.Context, writer client.Writer, obj client.Object, finalizers ...string) error {
	return client.IgnoreNotFound(patchFinalizers(ctx, writer, obj, mergeFromWithOptimisticLock, controllerutil.RemoveFinalizer, finalizers...))
}

// RemoveAllFinalizers ensures that the given object has no finalizers with exponential backoff.
// If any finalizers are set, it removes them and issues a patch.
func RemoveAllFinalizers(ctx context.Context, writer client.Writer, obj client.Object) error {
	beforePatch := obj.DeepCopyObject().(client.Object)
	obj.SetFinalizers(nil)
	return client.IgnoreNotFound(writer.Patch(ctx, obj, mergeFrom(beforePatch)))
}

func patchFinalizers(ctx context.Context, writer client.Writer, obj client.Object, patchFunc patchFn, mutate func(client.Object, string) bool, finalizers ...string) error {
	beforePatch := obj.DeepCopyObject().(client.Object)
	for _, finalizer := range finalizers {
		mutate(obj, finalizer)
	}
	return writer.Patch(ctx, obj, patchFunc(beforePatch))
}
