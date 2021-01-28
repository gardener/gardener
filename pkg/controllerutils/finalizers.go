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
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PatchFinalizers adds the given finalizers to the object via a patch request.
func PatchFinalizers(ctx context.Context, c client.Client, obj client.Object, finalizers ...string) error {
	beforePatch := obj.DeepCopyObject()

	for _, finalizer := range finalizers {
		controllerutil.AddFinalizer(obj, finalizer)
	}

	return c.Patch(ctx, obj, client.MergeFromWithOptions(beforePatch, client.MergeFromWithOptimisticLock{}))
}

// PatchRemoveFinalizers removes the given finalizers from the object via a patch request.
func PatchRemoveFinalizers(ctx context.Context, c client.Client, obj client.Object, finalizers ...string) error {
	beforePatch := obj.DeepCopyObject()

	for _, finalizer := range finalizers {
		controllerutil.RemoveFinalizer(obj, finalizer)
	}

	return c.Patch(ctx, obj, client.MergeFromWithOptions(beforePatch, client.MergeFromWithOptimisticLock{}))
}

// EnsureFinalizer ensure the <finalizer> is present for the object.
func EnsureFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	if err := kutil.TryUpdate(ctx, retry.DefaultBackoff, c, obj, func() error {
		controllerutil.AddFinalizer(obj, finalizer)
		return nil
	}); err != nil {
		return fmt.Errorf("could not ensure %q finalizer: %+v", finalizer, err)
	}
	return nil
}

// RemoveGardenerFinalizer removes the gardener finalizer from the object.
func RemoveGardenerFinalizer(ctx context.Context, c client.Client, obj client.Object) error {
	return RemoveFinalizer(ctx, c, obj, gardencorev1beta1.GardenerName)
}

// RemoveFinalizer removes the <finalizer> from the object.
func RemoveFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	if err := kutil.TryUpdate(ctx, retry.DefaultBackoff, c, obj, func() error {
		controllerutil.RemoveFinalizer(obj, finalizer)
		return nil
	}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not remove %q finalizer: %+v", finalizer, err)
	}

	// Wait until the above modifications are reflected in the cache to prevent unwanted reconcile
	// operations (sometimes the cache is not synced fast enough).
	pollerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return wait.PollImmediateUntil(time.Second, func() (bool, error) {
		err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if !controllerutil.ContainsFinalizer(obj, finalizer) {
			return true, nil
		}
		return false, nil
	}, pollerCtx.Done())
}
