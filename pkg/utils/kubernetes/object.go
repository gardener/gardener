// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

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

// ResourcesExist checks if there is at least one object of the given objList.
func ResourcesExist(ctx context.Context, reader client.Reader, objList client.ObjectList, scheme *runtime.Scheme, listOpts ...client.ListOption) (bool, error) {
	objects := objList

	// Use `PartialObjectMetadata` if no or metadata only field selectors are passed (informer's indexers only have access to metadata fields).
	if hasNoOrMetadataOnlyFieldSelector(listOpts...) {
		gvk, err := apiutil.GVKForObject(objList, scheme)
		if err != nil {
			return false, err
		}

		objects = &metav1.PartialObjectMetadataList{}
		objects.(*metav1.PartialObjectMetadataList).SetGroupVersionKind(gvk)
	}

	if err := reader.List(ctx, objects, append(listOpts, client.Limit(1))...); err != nil {
		return true, err
	}

	switch o := objects.(type) {
	case *metav1.PartialObjectMetadataList:
		return len(o.Items) > 0, nil
	default:
		items, err := meta.ExtractList(objList)
		if err != nil {
			return false, err
		}
		return len(items) > 0, err
	}
}

func hasNoOrMetadataOnlyFieldSelector(listOpts ...client.ListOption) bool {
	listOptions := &client.ListOptions{}
	for _, opt := range listOpts {
		opt.ApplyToList(listOptions)
	}

	if listOptions.FieldSelector == nil {
		return true
	}

	for _, req := range listOptions.FieldSelector.Requirements() {
		if !strings.HasPrefix(req.Field, "metadata") && req.Field != cache.NamespaceIndex {
			return false
		}
	}

	return true
}

// MakeUnique takes either a *corev1.ConfigMap or a *corev1.Secret object and makes it immutable, i.e., it sets
// .immutable=true, computes a checksum based on .data, and appends the first 8 characters of the computed checksum
// to the name of the object. Additionally, it injects the `resources.gardener.cloud/garbage-collectable-reference=true`
// label.
func MakeUnique(obj runtime.Object) error {
	var (
		numberOfChecksumChars = 8
		prependHyphen         = func(name string) string {
			if strings.HasSuffix(name, "-") {
				return ""
			}
			return "-"
		}
		mergeMaps = func(a map[string]string, b map[string][]byte) map[string][]byte {
			out := make(map[string][]byte, len(a)+len(b))

			for k, v := range a {
				out[k] = []byte(v)
			}
			for k, v := range b {
				out[k] = v
			}

			return out
		}
	)

	switch o := obj.(type) {
	case *corev1.Secret:
		o.Immutable = ptr.To(true)
		o.Name += prependHyphen(o.Name) + utils.ComputeSecretChecksum(mergeMaps(o.StringData, o.Data))[:numberOfChecksumChars]
		metav1.SetMetaDataLabel(&o.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)

	case *corev1.ConfigMap:
		o.Immutable = ptr.To(true)
		o.Name += prependHyphen(o.Name) + utils.ComputeSecretChecksum(mergeMaps(o.Data, o.BinaryData))[:numberOfChecksumChars]
		metav1.SetMetaDataLabel(&o.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)

	default:
		return fmt.Errorf("unhandled object type: %T", obj)
	}

	return nil
}
