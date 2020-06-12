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

package extensions

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
		return nil, fmt.Errorf("value of type %T does not implement Object", obj)
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

func nestedString(obj map[string]interface{}, fields ...string) string {
	v, ok, err := unstructured.NestedString(obj, fields...)
	if err != nil || !ok {
		return ""
	}
	return v
}

func nestedInt64(obj map[string]interface{}, fields ...string) int64 {
	v, ok, err := unstructured.NestedInt64(obj, fields...)
	if err != nil || !ok {
		return 0
	}
	return v
}

func nestedStringReference(obj map[string]interface{}, fields ...string) *string {
	v, ok, err := unstructured.NestedString(obj, fields...)
	if err != nil || !ok {
		return nil
	}

	return &v
}

func nestedRawExtension(obj map[string]interface{}, fields ...string) *runtime.RawExtension {
	val, ok, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !ok {
		return nil
	}

	data, ok := val.(map[string]interface{})
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
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(val.(map[string]interface{}), lastOperation); err != nil {
		return nil
	}
	return lastOperation
}

// GetLastError implements Status.
func (u unstructuredStatusAccessor) GetLastError() *gardencorev1beta1.LastError {
	val, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "lastError")
	if err != nil || !ok {
		return nil
	}

	lastError := &gardencorev1beta1.LastError{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(val.(map[string]interface{}), lastError); err != nil {
		return nil
	}
	return lastError
}

// GetObservedGeneration implements Status.
func (u unstructuredStatusAccessor) GetObservedGeneration() int64 {
	return nestedInt64(u.Object, "status", "observedGeneration")
}

// GetState implements Status.
func (u unstructuredStatusAccessor) GetState() *runtime.RawExtension {
	val, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "state")
	if err != nil || !ok {
		return nil
	}
	raw := &runtime.RawExtension{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(val.(map[string]interface{}), raw)
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
	interfaceConditionSlice := val.([]interface{})
	for _, interfaceCondition := range interfaceConditionSlice {
		new := interfaceCondition.(map[string]interface{})
		condition := &gardencorev1beta1.Condition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(new, condition); err != nil {
			return nil
		}
		conditions = append(conditions, *condition)
	}
	return conditions
}

// SetConditions implements Status.
func (u unstructuredStatusAccessor) SetConditions(conditions []gardencorev1beta1.Condition) {
	var interfaceSlice = make([]interface{}, len(conditions))
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
	interfaceResourceSlice := val.([]interface{})
	for _, interfaceResource := range interfaceResourceSlice {
		new := interfaceResource.(map[string]interface{})
		resource := &gardencorev1beta1.NamedResourceReference{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(new, resource); err != nil {
			return nil
		}
		resources = append(resources, *resource)
	}
	return resources
}

// SetResources implements Status.
func (u unstructuredStatusAccessor) SetResources(namedResourceReference []gardencorev1beta1.NamedResourceReference) {
	var interfaceSlice = make([]interface{}, len(namedResourceReference))
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
