// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	return patchFinalizers(ctx, resourcePatchFunc(writer), obj, controllerutil.AddFinalizer, finalizers...)
}

// RemoveFinalizers ensures that the given finalizer is not present anymore in the given object and optimistic locking.
// If it is set, it removes it and issues a patch.
// Note that this is done with a regular merge-patch since strategic merge-patches do not work with custom resources,
// see https://github.com/kubernetes/kubernetes/issues/105146.
func RemoveFinalizers(ctx context.Context, writer client.Writer, obj client.Object, finalizers ...string) error {
	return client.IgnoreNotFound(patchFinalizers(ctx, resourcePatchFunc(writer), obj, controllerutil.RemoveFinalizer, finalizers...))
}

// RemoveAllFinalizers ensures that the given object has no finalizers with exponential backoff.
// If any finalizers are set, it removes them and issues a patch.
func RemoveAllFinalizers(ctx context.Context, writer client.Writer, obj client.Object) error {
	beforePatch := obj.DeepCopyObject().(client.Object)
	obj.SetFinalizers(nil)
	return client.IgnoreNotFound(writer.Patch(ctx, obj, mergeFrom(beforePatch)))
}

// AddFinalizersViaMetadataSubresource is like AddFinalizers but issues the patch against the `metadata`
// subresource of the object instead of the main resource.
func AddFinalizersViaMetadataSubresource(ctx context.Context, c client.Client, obj client.Object, finalizers ...string) error {
	return patchFinalizers(ctx, metadataSubresourcePatchFunc(c), obj, controllerutil.AddFinalizer, finalizers...)
}

// RemoveFinalizersViaMetadataSubresource is like RemoveFinalizers but issues the patch against the `metadata`
// subresource of the object instead of the main resource.
func RemoveFinalizersViaMetadataSubresource(ctx context.Context, c client.Client, obj client.Object, finalizers ...string) error {
	return client.IgnoreNotFound(patchFinalizers(ctx, metadataSubresourcePatchFunc(c), obj, controllerutil.RemoveFinalizer, finalizers...))
}

func patchFinalizers(ctx context.Context, patch patchFunc, obj client.Object, mutate func(client.Object, string) bool, finalizers ...string) error {
	beforePatch := obj.DeepCopyObject().(client.Object)
	for _, finalizer := range finalizers {
		mutate(obj, finalizer)
	}
	return patch(ctx, obj, mergeFromWithOptimisticLock(beforePatch))
}

// patchFunc issues the given patch for the given object. resourcePatchFunc targets the main resource,
// metadataSubresourcePatchFunc the `metadata` subresource.
type patchFunc func(ctx context.Context, obj client.Object, patch client.Patch) error

func resourcePatchFunc(writer client.Writer) patchFunc {
	return func(ctx context.Context, obj client.Object, patch client.Patch) error {
		return writer.Patch(ctx, obj, patch)
	}
}

func metadataSubresourcePatchFunc(c client.Client) patchFunc {
	return func(ctx context.Context, obj client.Object, patch client.Patch) error {
		return c.SubResource("metadata").Patch(ctx, obj, patch)
	}
}
