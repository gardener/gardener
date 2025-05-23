// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Accessor tries to create an extensionsv1alpha1.Object from the given runtime.Object.
//
// If the given object already implements object, it is returned as-is.
// If the object is unstructured, an unstructured accessor is returned that retrieves values
// on a best-effort basis.
// Otherwise, an error with the type of the object is returned.
func Accessor(obj runtime.Object) (extensionsv1alpha1.Object, error) {
	switch v := obj.(type) {
	case extensionsv1alpha1.Object:
		return v, nil
	case *unstructured.Unstructured:
		return UnstructuredAccessor(v), nil
	default:
		return nil, fmt.Errorf("value of type %T does not implement extensionsv1alpha1.Object", obj)
	}
}

// UnstructuredAccessor is an Object that retrieves values on a best-effort basis.
// If values don't exist, it usually returns the zero value of them.
func UnstructuredAccessor(u *unstructured.Unstructured) extensionsv1alpha1.Object {
	return unstructuredAccessor{u}
}

type unstructuredAccessor struct {
	*unstructured.Unstructured
}

type unstructuredSpecAccessor struct {
	*unstructured.Unstructured
}

type unstructuredStatusAccessor struct {
	*unstructured.Unstructured
}

func nestedString(obj map[string]any, fields ...string) string {
	v, ok, err := unstructured.NestedString(obj, fields...)
	if err != nil || !ok {
		return ""
	}
	return v
}

func nestedInt64(obj map[string]any, fields ...string) int64 {
	v, ok, err := unstructured.NestedInt64(obj, fields...)
	if err != nil || !ok {
		return 0
	}
	return v
}

func nestedStringReference(obj map[string]any, fields ...string) *string {
	v, ok, err := unstructured.NestedString(obj, fields...)
	if err != nil || !ok {
		return nil
	}

	return &v
}

func nestedRawExtension(obj map[string]any, fields ...string) *runtime.RawExtension {
	val, ok, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !ok {
		return nil
	}

	data, ok := val.(map[string]any)
	if !ok {
		return nil
	}

	rawExtension := &runtime.RawExtension{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(data, rawExtension); err != nil {
		return nil
	}

	return rawExtension
}

// GetExtensionSpec implements Object.
func (u unstructuredAccessor) GetExtensionSpec() extensionsv1alpha1.Spec {
	return unstructuredSpecAccessor(u)
}

// GetExtensionType implements Spec.
func (u unstructuredSpecAccessor) GetExtensionType() string {
	return nestedString(u.UnstructuredContent(), "spec", "type")
}

// GetExtensionClass implements Spec.
func (u unstructuredSpecAccessor) GetExtensionClass() *extensionsv1alpha1.ExtensionClass {
	return (*extensionsv1alpha1.ExtensionClass)(nestedStringReference(u.UnstructuredContent(), "spec", "class"))
}

// GetExtensionPurpose implements Spec.
func (u unstructuredSpecAccessor) GetExtensionPurpose() *string {
	return nestedStringReference(u.UnstructuredContent(), "spec", "purpose")
}

// GetProviderConfig implements Spec.
func (u unstructuredSpecAccessor) GetProviderConfig() *runtime.RawExtension {
	return nestedRawExtension(u.UnstructuredContent(), "spec", "providerConfig")
}

// GetExtensionStatus implements Object.
func (u unstructuredAccessor) GetExtensionStatus() extensionsv1alpha1.Status {
	return unstructuredStatusAccessor(u)
}

// GetProviderStatus implements Status.
func (u unstructuredStatusAccessor) GetProviderStatus() *runtime.RawExtension {
	return nestedRawExtension(u.UnstructuredContent(), "status", "providerStatus")
}

// GetLastOperation implements Status.
func (u unstructuredStatusAccessor) GetLastOperation() *gardencorev1beta1.LastOperation {
	val, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "lastOperation")
	if err != nil || !ok {
		return nil
	}

	lastOperation := &gardencorev1beta1.LastOperation{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(val.(map[string]any), lastOperation); err != nil {
		return nil
	}
	return lastOperation
}

// SetLastOperation implements Status.
func (u unstructuredStatusAccessor) SetLastOperation(lastOp *gardencorev1beta1.LastOperation) {
	unstrc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(lastOp)
	if err != nil {
		return
	}

	if err := unstructured.SetNestedField(u.UnstructuredContent(), unstrc, "status", "lastOperation"); err != nil {
		return
	}
}

// GetLastError implements Status.
func (u unstructuredStatusAccessor) GetLastError() *gardencorev1beta1.LastError {
	val, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "lastError")
	if err != nil || !ok {
		return nil
	}

	lastError := &gardencorev1beta1.LastError{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(val.(map[string]any), lastError); err != nil {
		return nil
	}
	return lastError
}

// SetLastError implements Status.
func (u unstructuredStatusAccessor) SetLastError(lastErr *gardencorev1beta1.LastError) {
	unstrc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(lastErr)
	if err != nil {
		return
	}

	if err := unstructured.SetNestedField(u.UnstructuredContent(), unstrc, "status", "lastError"); err != nil {
		return
	}
}

// GetObservedGeneration implements Status.
func (u unstructuredStatusAccessor) GetObservedGeneration() int64 {
	return nestedInt64(u.Object, "status", "observedGeneration")
}

// SetObservedGeneration implements Status.
func (u unstructuredStatusAccessor) SetObservedGeneration(generation int64) {
	if err := unstructured.SetNestedField(u.UnstructuredContent(), generation, "status", "observedGeneration"); err != nil {
		return
	}
}

// GetState implements Status.
func (u unstructuredStatusAccessor) GetState() *runtime.RawExtension {
	val, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "state")
	if err != nil || !ok {
		return nil
	}
	raw := &runtime.RawExtension{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(val.(map[string]any), raw)
	if err != nil {
		return nil
	}
	return raw
}

// SetState implements Status.
func (u unstructuredStatusAccessor) SetState(state *runtime.RawExtension) {
	unstrc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(state)
	if err != nil {
		return
	}

	if err := unstructured.SetNestedField(u.UnstructuredContent(), unstrc, "status", "state"); err != nil {
		return
	}
}

// GetConditions implements Status.
func (u unstructuredStatusAccessor) GetConditions() []gardencorev1beta1.Condition {
	val, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "conditions")
	if err != nil || !ok {
		return nil
	}

	var conditions []gardencorev1beta1.Condition
	interfaceConditionSlice := val.([]any)
	for _, interfaceCondition := range interfaceConditionSlice {
		unstructuredCondition := interfaceCondition.(map[string]any)
		condition := &gardencorev1beta1.Condition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredCondition, condition); err != nil {
			return nil
		}
		conditions = append(conditions, *condition)
	}
	return conditions
}

// SetConditions implements Status.
func (u unstructuredStatusAccessor) SetConditions(conditions []gardencorev1beta1.Condition) {
	var interfaceSlice = make([]any, len(conditions))
	for i, d := range conditions {
		unstrc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
		if err != nil {
			return
		}
		interfaceSlice[i] = unstrc
	}
	err := unstructured.SetNestedSlice(u.UnstructuredContent(), interfaceSlice, "status", "conditions")
	if err != nil {
		return
	}
}

// GetResources implements Status.
func (u unstructuredStatusAccessor) GetResources() []gardencorev1beta1.NamedResourceReference {
	val, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "resources")
	if err != nil || !ok {
		return nil
	}
	var resources []gardencorev1beta1.NamedResourceReference
	interfaceResourceSlice := val.([]any)
	for _, interfaceResource := range interfaceResourceSlice {
		unstructuredResource := interfaceResource.(map[string]any)
		resource := &gardencorev1beta1.NamedResourceReference{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredResource, resource); err != nil {
			return nil
		}
		resources = append(resources, *resource)
	}
	return resources
}

// SetResources implements Status.
func (u unstructuredStatusAccessor) SetResources(namedResourceReference []gardencorev1beta1.NamedResourceReference) {
	var interfaceSlice = make([]any, len(namedResourceReference))
	for i, d := range namedResourceReference {
		unstrc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
		if err != nil {
			return
		}
		interfaceSlice[i] = unstrc
	}
	err := unstructured.SetNestedSlice(u.UnstructuredContent(), interfaceSlice, "status", "resources")
	if err != nil {
		return
	}
}
