// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"reflect"
	"strings"

	"github.com/gardener/gardener-resource-manager/pkg/controller/garbagecollector/references"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// ObjectName returns the name of the given object in the format <namespace>/<name>
func ObjectName(obj client.Object) string {
	if obj.GetNamespace() == "" {
		return obj.GetName()
	}
	return client.ObjectKeyFromObject(obj).String()
}

// DeleteObjects deletes a list of Kubernetes objects.
func DeleteObjects(ctx context.Context, c client.Writer, objects ...client.Object) error {
	for _, obj := range objects {
		if err := DeleteObject(ctx, c, obj); err != nil {
			return err
		}
	}
	return nil
}

// DeleteObject deletes a Kubernetes object. It ignores 'not found' and 'no match' errors.
func DeleteObject(ctx context.Context, c client.Writer, object client.Object) error {
	if err := c.Delete(ctx, object); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
		return err
	}
	return nil
}

// DeleteObjectsFromListConditionally takes a Kubernetes List object. It iterates over its items and, if provided,
// executes the predicate function. If it evaluates to true then the object will be deleted.
func DeleteObjectsFromListConditionally(ctx context.Context, c client.Client, listObj client.ObjectList, predicateFn func(runtime.Object) bool) error {
	fns := make([]flow.TaskFn, 0, meta.LenList(listObj))

	if err := meta.EachListItem(listObj, func(obj runtime.Object) error {
		if predicateFn == nil || predicateFn(obj) {
			fns = append(fns, func(ctx context.Context) error {
				return client.IgnoreNotFound(c.Delete(ctx, obj.(client.Object)))
			})
		}
		return nil
	}); err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// IsNamespaceInUse checks if there are is at least one object of the given kind left inside the given namespace.
func IsNamespaceInUse(ctx context.Context, reader client.Reader, namespace string, gvk schema.GroupVersionKind) (bool, error) {
	objects := &metav1.PartialObjectMetadataList{}
	objects.SetGroupVersionKind(gvk)

	if err := reader.List(ctx, objects, client.InNamespace(namespace), client.Limit(1)); err != nil {
		return true, err
	}

	return len(objects.Items) > 0, nil
}

// CreateResetObjectFunc creates a func that will reset the given object to a new empty object every time the func is called.
// This is useful for resetting an in-memory object before re-getting it from the API server / cache
// to avoid executing checks on stale/removed object data e.g. annotations/lastError
// (json decoder does not unset fields in the in-memory object that are unset in the API server's response)
func CreateResetObjectFunc(obj runtime.Object, scheme *runtime.Scheme) (func(), error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return nil, err
	}
	emptyObj, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	return func() {
		deepCopyIntoObject(obj, emptyObj)
	}, nil
}

// deepCopyIntoObject deep copies src into dest.
// This is a workaround for runtime.Object's lack of a DeepCopyInto method, similar to what the c-r cache does:
// https://github.com/kubernetes-sigs/controller-runtime/blob/55a329c15d6b4f91a9ff072fed6f6f05ff3339e7/pkg/cache/internal/cache_reader.go#L85-L90
func deepCopyIntoObject(dest, src runtime.Object) {
	reflect.ValueOf(dest).Elem().Set(reflect.ValueOf(src.DeepCopyObject()).Elem())
}

// MakeImmutable takes either a *corev1.ConfigMap or a *corev1.Secret object and makes it immutable, i.e., it sets
// .immutable=true, computes a checksum based on .data, and appends the first 8 characters of the computed checksum
// to the name of the object. Additionally, it injects the `resources.gardener.cloud/garbage-collectable-reference=true`
// label.
func MakeImmutable(obj runtime.Object) error {
	var (
		numberOfChecksumChars = 8
		prependHyphen         = func(name string) string {
			if strings.HasSuffix(name, "-") {
				return ""
			}
			return "-"
		}
	)

	switch o := obj.(type) {
	case *corev1.Secret:
		o.Immutable = pointer.Bool(true)
		o.Name += prependHyphen(o.Name) + utils.ComputeSecretChecksum(o.Data)[:numberOfChecksumChars]
		metav1.SetMetaDataLabel(&o.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)

	case *corev1.ConfigMap:
		o.Immutable = pointer.Bool(true)
		o.Name += prependHyphen(o.Name) + utils.ComputeConfigMapChecksum(o.Data)[:numberOfChecksumChars]
		metav1.SetMetaDataLabel(&o.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)

	default:
		return fmt.Errorf("unhandled object type: %T", obj)
	}

	return nil
}
