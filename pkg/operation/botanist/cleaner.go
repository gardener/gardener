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

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/gardener/gardener/pkg/utils/flow"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteAll calls delete for all objects in the given list.
//
// Not found errors are being ignored.
func DeleteAll(ctx context.Context, c client.Client, list runtime.Object, opts ...client.DeleteOptionFunc) error {
	fns := make([]flow.TaskFn, 0, meta.LenList(list))

	if err := meta.EachListItem(list, func(obj runtime.Object) error {
		fns = append(fns, func(ctx context.Context) error {
			if err := c.Delete(ctx, obj, opts...); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		})
		return nil
	}); err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// FinalizeAll iterates over the given list and removes the finalizers the individual objects, if any.
func FinalizeAll(ctx context.Context, c client.Client, list runtime.Object) error {
	var fns []flow.TaskFn

	if err := meta.EachListItem(list, func(obj runtime.Object) error {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return err
		}

		finalizers := accessor.GetFinalizers()
		if len(finalizers) == 0 {
			return nil
		}

		fns = append(fns, func(ctx context.Context) error {
			objCopy := obj.DeepCopyObject()
			copyAccessor, err := meta.Accessor(objCopy)
			if err != nil {
				return err
			}

			copyAccessor.SetFinalizers(nil)
			return c.Patch(ctx, obj, client.MergeFrom(objCopy))
		})
		return nil
	}); err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

type objectsRemaining []runtime.Object

var unknownKey = client.ObjectKey{Namespace: "<unknown>", Name: "<unknown>"}

// Error implements error.
func (n *objectsRemaining) Error() string {
	out := make([]string, 0, len(*n))
	for _, obj := range *n {
		key, err := client.ObjectKeyFromObject(obj)
		if err != nil {
			key = unknownKey
		}

		var typeID string
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			typeID = fmt.Sprintf("%T", (*n)[0])
		} else {
			typeID = gvk.String()
		}

		out = append(out, fmt.Sprintf("%s %s", typeID, key.String()))
	}
	return fmt.Sprintf("remaining objects are still present: %v", out)
}

// AreObjectsRemaining checks whether the given error is an 'objects remaining error'.
func AreObjectsRemaining(err error) bool {
	_, ok := err.(*objectsRemaining)
	return ok
}

// NewObjectsRemaining returns a new error with the remaining objects.
func NewObjectsRemaining(remaining []runtime.Object) error {
	err := objectsRemaining(remaining)
	return &err
}

// RemainingObjects retrieves the remaining objects of an `AreObjectsRemaining` error.
//
// If the error does not match `AreObjectsRemaining`, this returns nil.
func RemainingObjects(err error) []runtime.Object {
	if nEmpty, ok := err.(*objectsRemaining); ok {
		return *nEmpty
	}
	return nil
}

// CheckObjectsRemaining checks if the given list is empty.
//
// Iff it is not, returns a `NewObjectsRemaining` error with the remaining objects.
func CheckObjectsRemaining(list runtime.Object) error {
	if n := meta.LenList(list); n > 0 {
		remaining, err := meta.ExtractList(list)
		if err != nil {
			return err
		}

		return NewObjectsRemaining(remaining)
	}
	return nil
}

// CheckObjectsRemainingMatching calls the client and checks if there are objects remaining matching the given opts.
func CheckObjectsRemainingMatching(
	ctx context.Context,
	c client.Client,
	list runtime.Object,
	opts ...client.ListOptionFunc,
) error {
	if err := c.List(ctx, list, opts...); err != nil {
		return err
	}

	return CheckObjectsRemaining(list)
}

// DeleteMatching issues DELETE calls to all remote objects that match the given selector.
//
// If `finalize` is set, this also removes all finalizers from the matching objects before deleting them.
func DeleteMatching(
	ctx context.Context,
	c client.Client,
	list runtime.Object,
	opts ...CleanOptionFunc,
) error {
	cleanOptions := &CleanOptions{}
	cleanOptions.ApplyOptions(opts)
	if err := c.List(ctx, list, cleanOptions.ListOpts...); err != nil {
		return err
	}

	if meta.LenList(list) == 0 {
		return nil
	}

	if cleanOptions.Finalize {
		if err := FinalizeAll(ctx, c, list); err != nil {
			return err
		}
	}

	// TODO: Make this a `DeleteCollection` as soon it is in the controller-runtime:
	// https://github.com/kubernetes-sigs/controller-runtime/pull/324
	return DeleteAll(ctx, c, list, cleanOptions.DeleteOpts...)
}

// CleanOptions are options for cleaning a set of resources.
// TODO: Adapt / remove this once `DeleteCollection` is in the controller-runtime.
type CleanOptions struct {
	Finalize   bool
	DeleteOpts []client.DeleteOptionFunc
	ListOpts   []client.ListOptionFunc
}

// ApplyOptions applies the given optFuncs to the CleanOptions.
func (o *CleanOptions) ApplyOptions(optFuncs []CleanOptionFunc) {
	for _, optFunc := range optFuncs {
		optFunc(o)
	}
}

// CleanOptionFunc is a function that modifies the given CleanOptions.
type CleanOptionFunc func(*CleanOptions)

// DeleteOptions creates a CleanOptionFunc that adds all the DeleteOptionFuncs to the CleanOptions.
func DeleteOptions(deleteOpts ...client.DeleteOptionFunc) CleanOptionFunc {
	return func(opts *CleanOptions) {
		opts.DeleteOpts = append(opts.DeleteOpts, deleteOpts...)
	}
}

// ListOptions creates a CleanOptionFunc that adds all the ListOptionFuncs to the CleanOptions.
func ListOptions(listOpts ...client.ListOptionFunc) CleanOptionFunc {
	return func(opts *CleanOptions) {
		opts.ListOpts = append(opts.ListOpts, listOpts...)
	}
}

// Finalize enforces finalizing objects before cleaning them.
var Finalize = func(o *CleanOptions) {
	o.Finalize = true
}

// CleanMatching deletes all objects matching `deleteOpts`, then it checks if there are no objects left matching `checkOpts`.
func CleanMatching(
	ctx context.Context,
	c client.Client,
	list runtime.Object,
	opts ...CleanOptionFunc,
) error {
	if err := DeleteMatching(ctx, c, list, opts...); err != nil {
		return err
	}

	cleanOptions := &CleanOptions{}
	cleanOptions.ApplyOptions(opts)

	return CheckObjectsRemainingMatching(ctx, c, list, cleanOptions.ListOpts...)
}

// RetryCleanMatchingUntil repeatedly tries to `CleanMatching` objects.
func RetryCleanMatchingUntil(
	ctx context.Context,
	interval time.Duration,
	c client.Client,
	list runtime.Object,
	opts ...CleanOptionFunc,
) error {
	return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		if err := CleanMatching(ctx, c, list, opts...); err != nil {
			if AreObjectsRemaining(err) {
				return retry.MinorError(err)
			}
			return retry.SevereError(err)
		}
		return retry.Ok()
	})
}
