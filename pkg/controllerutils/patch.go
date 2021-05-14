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

// MergePatchOrCreate patches (using a merge patch) or creates the given object in the Kubernetes cluster.
// The object's desired state is only reconciled with the existing state inside the passed in callback MutateFn,
// however, the object is not read from the client. This means the object should already be filled with the
// last-known state if operating on more complex structures (e.g. if the patch is supposed to remove an optional field
// or section). If you don't have the current state of an object, use GetAndCreateOrMergePatch instead.
//
// The MutateFn is called regardless of creating or patching an object.
//
// It returns the executed operation and an error.
func MergePatchOrCreate(ctx context.Context, c client.Writer, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return patchOrCreate(ctx, c, obj, func(obj client.Object) client.Patch {
		return client.MergeFrom(obj)
	}, f)
}

// StrategicMergePatchOrCreate patches (using a strategic merge patch) or creates the given object in the Kubernetes cluster.
// The object's desired state is only reconciled with the existing state inside the passed in callback MutateFn,
// however, the object is not read from the client. This means the object should already be filled with the
// last-known state if operating on more complex structures (e.g. if the patch is supposed to remove an optional field
// or section). If you don't have the current state of an object, use GetAndCreateOrStrategicMergePatch instead.
//
// The MutateFn is called regardless of creating or patching an object.
//
// It returns the executed operation and an error.
func StrategicMergePatchOrCreate(ctx context.Context, c client.Writer, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return patchOrCreate(ctx, c, obj, func(obj client.Object) client.Patch {
		return client.StrategicMergeFrom(obj)
	}, f)
}

func patchOrCreate(ctx context.Context, c client.Writer, obj client.Object, patchFunc func(client.Object) client.Patch, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	patch := patchFunc(obj.DeepCopyObject().(client.Object))

	if err := f(); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if err := c.Patch(ctx, obj, patch); err != nil {
		if !apierrors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}

		if err2 := c.Create(ctx, obj); err2 != nil {
			return controllerutil.OperationResultNone, err2
		}

		return controllerutil.OperationResultCreated, nil
	}

	return controllerutil.OperationResultUpdated, nil
}

// GetAndCreateOrMergePatch is similar to controllerutil.CreateOrPatch, but does not care about the object's status section.
// It reads the object from the client, reconciles the desired state with the existing state using the given MutateFn
// and creates or patches the object (using a merge patch) accordingly.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func GetAndCreateOrMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return getAndCreateOrPatch(ctx, c, obj, func(obj client.Object) client.Patch {
		return client.MergeFrom(obj)
	}, f)
}

// GetAndCreateOrStrategicMergePatch is similar to controllerutil.CreateOrPatch, but does not care about the object's status section.
// It reads the object from the client, reconciles the desired state with the existing state using the given MutateFn
// and creates or patches the object (using a strategic merge patch) accordingly.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func GetAndCreateOrStrategicMergePatch(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	return getAndCreateOrPatch(ctx, c, obj, func(obj client.Object) client.Patch {
		return client.StrategicMergeFrom(obj)
	}, f)
}

func getAndCreateOrPatch(ctx context.Context, c client.Client, obj client.Object, patchFunc func(client.Object) client.Patch, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
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
