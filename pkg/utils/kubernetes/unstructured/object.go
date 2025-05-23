// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package unstructured

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

var systemMetadataFields = []string{"ownerReferences", "uid", "resourceVersion", "generation", "creationTimestamp", "deletionTimestamp", "deletionGracePeriodSeconds", "managedFields"}

// GetObjectByRef returns the object with the given reference and namespace using the given client.
// The full content of the object is returned as map[string]any, except for system metadata fields.
// This function can be combined with runtime.DefaultUnstructuredConverter.FromUnstructured to get the object content
// as runtime.RawExtension.
func GetObjectByRef(ctx context.Context, c client.Client, ref *autoscalingv1.CrossVersionObjectReference, namespace string) (map[string]any, error) {
	gvk, err := gvkFromCrossVersionObjectReference(ref)
	if err != nil {
		return nil, err
	}
	return GetObject(ctx, c, gvk, ref.Name, namespace)
}

// GetObject returns the object with the given GVK, name, and namespace as a map using the given client.
// The full content of the object is returned as map[string]any, except for system metadata fields.
// This function can be combined with runtime.DefaultUnstructuredConverter.FromUnstructured to get the object content
// as runtime.RawExtension.
func GetObject(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, name, namespace string) (map[string]any, error) {
	// Initialize the object
	key := client.ObjectKey{Namespace: namespace, Name: name}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	// Get the object
	if err := c.Get(ctx, key, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}

	// Return object content
	return FilterMetadata(obj.UnstructuredContent(), systemMetadataFields...), nil
}

// CreateOrPatchObjectByRef creates or patches the object with the given reference and namespace using the given client.
// The object is created or patched with the given content, except for system metadata fields.
// This function can be combined with runtime.DefaultUnstructuredConverter.ToUnstructured to create or update an object
// from runtime.RawExtension.
func CreateOrPatchObjectByRef(ctx context.Context, c client.Client, ref *autoscalingv1.CrossVersionObjectReference, namespace string, content map[string]any) error {
	gvk, err := gvkFromCrossVersionObjectReference(ref)
	if err != nil {
		return err
	}
	return CreateOrPatchObject(ctx, c, gvk, ref.Name, namespace, content)
}

// CreateOrPatchObject creates or patches the object with the given GVK, name, and namespace using the given client.
// The object is created or patched with the given content, except for system metadata fields, namespace, and name.
// This function can be combined with runtime.DefaultUnstructuredConverter.ToUnstructured to create or update an object
// from runtime.RawExtension.
func CreateOrPatchObject(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, name, namespace string, content map[string]any) error {
	// Initialize the object
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespace)

	// Create or patch the object
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, obj, func() error {
		// Set object content
		if content != nil {
			obj.SetUnstructuredContent(mergeObjectContents(obj.UnstructuredContent(),
				FilterMetadata(content, add(systemMetadataFields, "namespace", "name")...)))
		}
		return nil
	})
	return err
}

// DeleteObjectByRef deletes the object with the given reference and namespace using the given client.
func DeleteObjectByRef(ctx context.Context, c client.Client, ref *autoscalingv1.CrossVersionObjectReference, namespace string) error {
	gvk, err := gvkFromCrossVersionObjectReference(ref)
	if err != nil {
		return err
	}
	return DeleteObject(ctx, c, gvk, ref.Name, namespace)
}

// DeleteObject deletes the object with the given GVK, name, and namespace using the given client.
func DeleteObject(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, name, namespace string) error {
	// Initialize the object
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespace)

	// Delete the object
	return client.IgnoreNotFound(c.Delete(ctx, obj))
}

func gvkFromCrossVersionObjectReference(ref *autoscalingv1.CrossVersionObjectReference) (schema.GroupVersionKind, error) {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    ref.Kind,
	}, nil
}

func mergeObjectContents(dest, src map[string]any) map[string]any {
	// Merge metadata
	srcMetadata, srcMetadataOK := src["metadata"].(map[string]any)
	if srcMetadataOK {
		destMetadata, destMetadataOK := dest["metadata"].(map[string]any)
		if destMetadataOK {
			dest["metadata"] = utils.MergeMaps(destMetadata, srcMetadata)
		} else {
			dest["metadata"] = srcMetadata
		}
	}

	// Take spec and data from the source
	for _, key := range []string{"spec", "data", "stringData"} {
		srcSpec, srcSpecOK := src[key]
		if srcSpecOK {
			dest[key] = srcSpec
		} else {
			delete(dest, key)
		}
	}

	return dest
}

// FilterMetadata filters metadata from the provided unstructured object content.
func FilterMetadata(content map[string]any, fields ...string) map[string]any {
	// Copy content to result
	result := make(map[string]any)
	for key, value := range content {
		result[key] = value
	}

	// Delete specified fields from result
	if metadata, ok := result["metadata"].(map[string]any); ok {
		for _, field := range fields {
			delete(metadata, field)
		}
	}
	return result
}

func add(s []string, elems ...string) []string {
	result := make([]string, len(s))
	copy(result, s)
	return append(result, elems...)
}
