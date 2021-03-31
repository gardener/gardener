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

// PatchOrCreate patches or creates the given object in the Kubernetes cluster. The object's desired state is only
// reconciled with the existing state inside the passed in callback MutateFn, however, the object is not read from the
// API server.
//
// The MutateFn is called regardless of creating or patching an object.
//
// It returns the executed operation and an error.
func PatchOrCreate(ctx context.Context, c client.Writer, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	patch := client.StrategicMergeFrom(obj.DeepCopyObject().(client.Object))

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
