// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerutils

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PatchAddFinalizers adds the given finalizers to the object via a patch request.
func PatchAddFinalizers(ctx context.Context, writer client.Writer, obj client.Object, finalizers ...string) error {
	return patchFinalizers(ctx, writer, obj, controllerutil.AddFinalizer, finalizers...)
}

// PatchRemoveFinalizers removes the given finalizers from the object via a patch request.
func PatchRemoveFinalizers(ctx context.Context, writer client.Writer, obj client.Object, finalizers ...string) error {
	return patchFinalizers(ctx, writer, obj, controllerutil.RemoveFinalizer, finalizers...)
}

func patchFinalizers(ctx context.Context, writer client.Writer, obj client.Object, mutate func(client.Object, string), finalizers ...string) error {
	beforePatch := obj.DeepCopyObject().(client.Object)
	for _, finalizer := range finalizers {
		mutate(obj, finalizer)
	}

	return writer.Patch(ctx, obj, client.MergeFromWithOptions(beforePatch, client.MergeFromWithOptimisticLock{}))
}

// EnsureFinalizer ensures that a finalizer of the given name is set on the given object with exponential backoff.
// If the finalizer is not set, it adds it to the list of finalizers and patches the remote object.
// Use PatchAddFinalizers instead, if the controller is able to tolerate conflict errors caused by stale reads.
func EnsureFinalizer(ctx context.Context, reader client.Reader, writer client.Writer, obj client.Object, finalizer string) error {
	return tryPatchFinalizers(ctx, reader, writer, obj, controllerutil.AddFinalizer, finalizer)
}

// RemoveFinalizer ensures that the given finalizer is not present anymore in the given object with exponential backoff.
// If it is set, it removes it and issues a patch.
// Use PatchRemoveFinalizers instead, if the controller is able to tolerate conflict errors caused by stale reads.
func RemoveFinalizer(ctx context.Context, reader client.Reader, writer client.Writer, obj client.Object, finalizer string) error {
	return tryPatchFinalizers(ctx, reader, writer, obj, controllerutil.RemoveFinalizer, finalizer)
}

func tryPatchFinalizers(ctx context.Context, reader client.Reader, writer client.Writer, obj client.Object, mutate func(client.Object, string), finalizer string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Unset finalizers array manually here, because finalizers array won't be unset in decoder, if it's empty on
		// the API server. This can lead to an empty patch, although we want to ensure, that the finalizer is present.
		// The patch itself will go through and it won't be noticed that the finalizer wasn't added at all
		obj.SetFinalizers(nil)

		if err := reader.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return err
		}

		return patchFinalizers(ctx, writer, obj, mutate, finalizer)
	})
}
