// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TryUpdate tries to apply the given transformation function onto the given object, and to update it afterwards.
// It retries the update with an exponential backoff.
func TryUpdate(ctx context.Context, backoff wait.Backoff, c client.Client, obj runtime.Object, transform func() error) error {
	return tryUpdate(ctx, backoff, c, obj, c.Update, transform)
}

// TryUpdateStatus tries to apply the given transformation function onto the given object, and to update its
// status afterwards. It retries the status update with an exponential backoff.
func TryUpdateStatus(ctx context.Context, backoff wait.Backoff, c client.Client, obj runtime.Object, transform func() error) error {
	return tryUpdate(ctx, backoff, c, obj, c.Status().Update, transform)
}

func tryUpdate(ctx context.Context, backoff wait.Backoff, c client.Client, obj runtime.Object, updateFunc func(context.Context, runtime.Object, ...client.UpdateOption) error, transform func() error) error {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	return exponentialBackoff(ctx, backoff, func() (bool, error) {
		if err := c.Get(ctx, key, obj); err != nil {
			return false, err
		}

		beforeTransform := obj.DeepCopyObject()
		if err := transform(); err != nil {
			return false, err
		}

		if reflect.DeepEqual(obj, beforeTransform) {
			return true, nil
		}

		if err := updateFunc(ctx, obj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

func exponentialBackoff(ctx context.Context, backoff wait.Backoff, condition wait.ConditionFunc) error {
	duration := backoff.Duration

	for i := 0; i < backoff.Steps; i++ {
		if ok, err := condition(); err != nil || ok {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			adjusted := duration
			if backoff.Jitter > 0.0 {
				adjusted = wait.Jitter(duration, backoff.Jitter)
			}
			time.Sleep(adjusted)
			duration = time.Duration(float64(duration) * backoff.Factor)
		}

		i++
	}

	return wait.ErrWaitTimeout
}
