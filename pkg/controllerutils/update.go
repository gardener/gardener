// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TypedCreateOrUpdate is like controllerutil.CreateOrUpdate, it retrieves the current state of the object from the
// API server, applies the given mutate func and creates or updates it afterwards. In contrast to
// controllerutil.CreateOrUpdate it tries to create a new typed object of obj's kind (using the provided scheme)
// to make typed Get requests in order to leverage the client's cache.
func TypedCreateOrUpdate(ctx context.Context, c client.Client, scheme *runtime.Scheme, obj *unstructured.Unstructured, alwaysUpdate bool, mutate func() error) (controllerutil.OperationResult, error) {
	// client.DelegatingReader does not use its cache for unstructured.Unstructured objects, so we
	// create a new typed object of the object's type to use the cache for get calls before applying changes
	var current client.Object
	if typed, err := scheme.New(obj.GetObjectKind().GroupVersionKind()); err == nil {
		var ok bool
		current, ok = typed.(client.Object)
		if !ok {
			return controllerutil.OperationResultNone, fmt.Errorf("object type %q is unsupported", obj.GetObjectKind().GroupVersionKind().String())
		}
	} else {
		// fallback to unstructured request (type might not be registered in scheme)
		current = obj
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), current); err != nil {
		if !apierrors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if err := mutate(); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, c.Create(ctx, obj)
	}

	var existing *unstructured.Unstructured

	// convert object back to unstructured for easy mutating/merging
	if _, isUnstructured := current.(*unstructured.Unstructured); !isUnstructured {
		u := &unstructured.Unstructured{}
		if err := scheme.Convert(current, u, nil); err != nil {
			return controllerutil.OperationResultNone, err
		}
		u.DeepCopyInto(obj)
		existing = u
	} else {
		existing = obj.DeepCopy()
	}

	if err := mutate(); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if !alwaysUpdate && apiequality.Semantic.DeepEqual(existing, obj) {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.OperationResultUpdated, c.Update(ctx, obj)
}
