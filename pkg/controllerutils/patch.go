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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// patchFn returns a client.Patch with the given client.Object as the base object.
type patchFn func(client.Object, ...client.MergeFromOption) client.Patch

func mergeFrom(obj client.Object, opts ...client.MergeFromOption) client.Patch {
	return client.MergeFromWithOptions(obj, opts...)
}

func mergeFromWithOptimisticLock(obj client.Object, opts ...client.MergeFromOption) client.Patch {
	return client.MergeFromWithOptions(obj, append(opts, client.MergeFromWithOptimisticLock{})...)
}

func strategicMergeFrom(obj client.Object, opts ...client.MergeFromOption) client.Patch {
	return client.StrategicMergeFrom(obj, opts...)
}

// GetAndCreateOrMergePatch is similar to controllerutil.CreateOrPatch, but does not care about the object's status section.
// It reads the object from the client, reconciles the desired state with the existing state using the given MutateFn
// and creates or patches the object (using a merge patch) accordingly.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func GetAndCreateOrMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn, opts ...client.MergeFromOption) (controllerutil.OperationResult, error) {
	return getAndCreateOrPatch(ctx, c, obj, mergeFrom, f, opts...)
}

// GetAndCreateOrStrategicMergePatch is similar to controllerutil.CreateOrPatch, but does not care about the object's status section.
// It reads the object from the client, reconciles the desired state with the existing state using the given MutateFn
// and creates or patches the object (using a strategic merge patch) accordingly.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func GetAndCreateOrStrategicMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn, opts ...client.MergeFromOption) (controllerutil.OperationResult, error) {
	return getAndCreateOrPatch(ctx, c, obj, strategicMergeFrom, f, opts...)
}

func getAndCreateOrPatch(ctx context.Context, c client.Client, obj client.Object, patchFunc patchFn, f controllerutil.MutateFn, opts ...client.MergeFromOption) (controllerutil.OperationResult, error) {
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

	patch := patchFunc(obj.DeepCopyObject().(client.Object), opts...)
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
func CreateOrGetAndMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn, opts ...client.MergeFromOption) (controllerutil.OperationResult, error) {
	return createOrGetAndPatch(ctx, c, obj, mergeFrom, f, opts...)
}

// CreateOrGetAndStrategicMergePatch creates or gets and patches (using a strategic merge patch) the given object in the Kubernetes cluster.
//
// The MutateFn is called regardless of creating or patching an object.
//
// It returns the executed operation and an error.
func CreateOrGetAndStrategicMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn, opts ...client.MergeFromOption) (controllerutil.OperationResult, error) {
	return createOrGetAndPatch(ctx, c, obj, strategicMergeFrom, f, opts...)
}

func createOrGetAndPatch(ctx context.Context, c client.Client, obj client.Object, patchFunc patchFn, f controllerutil.MutateFn, opts ...client.MergeFromOption) (controllerutil.OperationResult, error) {
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

		patch := patchFunc(obj.DeepCopyObject().(client.Object), opts...)
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
