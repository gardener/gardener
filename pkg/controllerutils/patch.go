// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// patchFn returns a client.Patch with the given client.Object as the base object.
type patchFn func(client.Object) client.Patch

func mergeFrom(obj client.Object) client.Patch {
	return client.MergeFrom(obj)
}

func mergeFromWithOptimisticLock(obj client.Object) client.Patch {
	return client.MergeFromWithOptions(obj, client.MergeFromWithOptimisticLock{})
}

func strategicMergeFrom(obj client.Object) client.Patch {
	return client.StrategicMergeFrom(obj)
}

// GetAndCreateOrMergePatch is similar to controllerutil.CreateOrPatch, but does not care about the object's status section.
// It reads the object from the client, reconciles the desired state with the existing state using the given MutateFn
// and creates or patches the object (using a merge patch) accordingly.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func GetAndCreateOrMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return getAndCreateOrPatch(ctx, c, obj, mergeFrom, f)
}

// GetAndCreateOrStrategicMergePatch is similar to controllerutil.CreateOrPatch, but does not care about the object's status section.
// It reads the object from the client, reconciles the desired state with the existing state using the given MutateFn
// and creates or patches the object (using a strategic merge patch) accordingly.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func GetAndCreateOrStrategicMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return getAndCreateOrPatch(ctx, c, obj, strategicMergeFrom, f)
}

func getAndCreateOrPatch(ctx context.Context, c client.Client, obj client.Object, patchFunc patchFn, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := c.Get(ctx, key, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if err := f(); err != nil {
			return controllerutil.OperationResultNone, err
		}
		if err := c.Create(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	patch := patchFunc(obj.DeepCopyObject().(client.Object))
	if err := f(); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if err := c.Patch(ctx, obj, patch); err != nil {
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.OperationResultUpdated, nil
}

// CreateOrGetAndMergePatch creates or gets and patches (using a merge patch) the given object in the Kubernetes cluster.
//
// The MutateFn is called regardless of creating or patching an object.
//
// It returns the executed operation and an error.
func CreateOrGetAndMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return createOrGetAndPatch(ctx, c, obj, mergeFrom, f)
}

// CreateOrGetAndStrategicMergePatch creates or gets and patches (using a strategic merge patch) the given object in the Kubernetes cluster.
//
// The MutateFn is called regardless of creating or patching an object.
//
// It returns the executed operation and an error.
func CreateOrGetAndStrategicMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return createOrGetAndPatch(ctx, c, obj, strategicMergeFrom, f)
}

func createOrGetAndPatch(ctx context.Context, c client.Client, obj client.Object, patchFunc patchFn, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	if err := f(); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if err := c.Create(ctx, obj); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return controllerutil.OperationResultNone, err
		}

		if err2 := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err2 != nil {
			return controllerutil.OperationResultNone, err2
		}

		patch := patchFunc(obj.DeepCopyObject().(client.Object))
		if err2 := f(); err2 != nil {
			return controllerutil.OperationResultNone, err2
		}

		if err2 := c.Patch(ctx, obj, patch); err2 != nil {
			return controllerutil.OperationResultNone, err2
		}

		return controllerutil.OperationResultUpdated, nil
	}

	return controllerutil.OperationResultCreated, nil
}

// TryPatch tries to apply the given transformation function onto the given object, and to patch it afterwards with optimistic locking.
// It retries the patch with an exponential backoff.
// Deprecated: This function is deprecated and will be removed in a future version. Please don't consider using it.
// See https://github.com/gardener/gardener/blob/master/docs/development/kubernetes-clients.md#dont-retry-on-conflict
// for more information.
func TryPatch(ctx context.Context, backoff wait.Backoff, c client.Client, obj client.Object, transform func() error) error {
	return tryPatch(ctx, backoff, c, obj, c.Patch, transform)
}

// TryPatchStatus tries to apply the given transformation function onto the given object, and to patch its
// status afterwards with optimistic locking. It retries the status patch with an exponential backoff.
// Deprecated: This function is deprecated and will be removed in a future version. Please don't consider using it.
// See https://github.com/gardener/gardener/blob/master/docs/development/kubernetes-clients.md#dont-retry-on-conflict
// for more information.
func TryPatchStatus(ctx context.Context, backoff wait.Backoff, c client.Client, obj client.Object, transform func() error) error {
	return tryPatch(ctx, backoff, c, obj, c.Status().Patch, transform)
}

func tryPatch(ctx context.Context, backoff wait.Backoff, c client.Client, obj client.Object, patchFunc func(context.Context, client.Object, client.Patch, ...client.PatchOption) error, transform func() error) error {
	resetCopy := obj.DeepCopyObject()
	return exponentialBackoff(ctx, backoff, func() (bool, error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return false, err
		}
		beforeTransform := obj.DeepCopyObject().(client.Object)
		if err := transform(); err != nil {
			return false, err
		}

		if reflect.DeepEqual(obj, beforeTransform) {
			return true, nil
		}

		patch := client.MergeFromWithOptions(beforeTransform, client.MergeFromWithOptimisticLock{})

		if err := patchFunc(ctx, obj, patch); err != nil {
			if apierrors.IsConflict(err) {
				reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(resetCopy).Elem())
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}
