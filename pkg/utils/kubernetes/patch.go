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

package kubernetes

import (
	"context"
	"reflect"
	"strings"

	jsoniter "github.com/json-iterator/go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var json = jsoniter.ConfigFastest

// TryPatch tries to apply the given transformation function onto the given object, and to patch it afterwards with optimistic locking.
// It retries the patch with an exponential backoff.
func TryPatch(ctx context.Context, backoff wait.Backoff, c client.Client, obj client.Object, transform func() error) error {
	return tryPatch(ctx, backoff, c, obj, c.Patch, transform)
}

// TryPatchStatus tries to apply the given transformation function onto the given object, and to patch its
// status afterwards with optimistic locking. It retries the status patch with an exponential backoff.
func TryPatchStatus(ctx context.Context, backoff wait.Backoff, c client.Client, obj client.Object, transform func() error) error {
	return tryPatch(ctx, backoff, c, obj, c.Status().Patch, transform)
}

func tryPatch(ctx context.Context, backoff wait.Backoff, c client.Client, obj client.Object, patchFunc func(context.Context, client.Object, client.Patch, ...client.PatchOption) error, transform func() error) error {
	resetCopy := obj.DeepCopyObject()
	return exponentialBackoff(ctx, backoff, func() (bool, error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return false, err
		}
		beforeTransform := obj.DeepCopyObject()
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

// IsEmptyPatch checks if the given patch is empty. A patch is considered empty if it is
// the empty string or if it json-decodes to an empty json map.
func IsEmptyPatch(patch []byte) bool {
	if len(strings.TrimSpace(string(patch))) == 0 {
		return true
	}

	var m map[string]interface{}
	if err := json.Unmarshal(patch, &m); err != nil {
		return false
	}

	return len(m) == 0
}

// SubmitEmptyPatch submits an empty patch to the given `obj` with the given `client` instance.
func SubmitEmptyPatch(ctx context.Context, c client.Client, obj client.Object) error {
	return c.Patch(ctx, obj, client.RawPatch(types.StrategicMergePatchType, []byte("{}")))
}
