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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type unstructuredStatusAccessor struct {
	*unstructured.Unstructured
}

type unstructuredLastOperationAccessor struct {
	*unstructured.Unstructured
}

type unstructuredLastErrorAccessor struct {
	*unstructured.Unstructured
}

type unstructuredSpecAccessor struct {
	*unstructured.Unstructured
}

func nestedStringSlice(obj map[string]interface{}, fields ...string) []string {
	v, ok, err := unstructured.NestedStringSlice(obj, fields...)
	if err != nil || !ok {
		return nil
	}
	return v
}

func nestedString(obj map[string]interface{}, fields ...string) string {
	v, ok, err := unstructured.NestedString(obj, fields...)
	if err != nil || !ok {
		return ""
	}
	return v
}

func nestedInt32(obj map[string]interface{}, fields ...string) int32 {
	v, ok, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !ok {
		return 0
	}

	switch x := v.(type) {
	case int64:
		// safe, as the DefaultUnstructuredConverter uses int64 to store int16, int32, etc.
		return int32(x)
	case int32:
		return x
	case int:
		return int32(x)
	default:
		return 0
	}
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

// GetDescription implements LastOperation.
func (u unstructuredLastOperationAccessor) GetDescription() string {
	return nestedString(u.UnstructuredContent(), "status", "lastOperation", "description")
}

// GetLastUpdateTime implements LastOperation.
func (u unstructuredLastOperationAccessor) GetLastUpdateTime() metav1.Time {
	var timestamp metav1.Time
	_ = timestamp.UnmarshalQueryParameter(nestedString(u.UnstructuredContent(), "status", "lastOperation", "lastUpdateTime"))
	return timestamp
}

// GetProgress implements LastOperation.
func (u unstructuredLastOperationAccessor) GetProgress() int32 {
	return nestedInt32(u.UnstructuredContent(), "status", "lastOperation", "progress")
}

// GetState implements LastOperation.
func (u unstructuredLastOperationAccessor) GetState() gardencorev1beta1.LastOperationState {
	return gardencorev1beta1.LastOperationState(nestedString(u.UnstructuredContent(), "status", "lastOperation", "state"))
}

// GetType implements LastOperation.
func (u unstructuredLastOperationAccessor) GetType() gardencorev1beta1.LastOperationType {
	return gardencorev1beta1.LastOperationType(nestedString(u.UnstructuredContent(), "status", "lastOperation", "type"))
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

// GetProviderStatus implements Status.
func (u unstructuredStatusAccessor) GetProviderStatus() *runtime.RawExtension {
	return nestedRawExtension(u.UnstructuredContent(), "status", "providerStatus")
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

// GetLastOperation implements Status.
func (u unstructuredStatusAccessor) GetLastOperation() extensionsv1alpha1.LastOperation {
	if _, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "lastOperation"); err != nil || !ok {
		return nil
	}
	return unstructuredLastOperationAccessor(u)
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

// GetDescription implements LastError.
func (u unstructuredLastErrorAccessor) GetDescription() string {
	return nestedString(u.Object, "status", "lastError", "description")
}

// GetTaskID implements LastError
func (u unstructuredLastErrorAccessor) GetTaskID() *string {
	s, ok, err := unstructured.NestedString(u.Object, "status", "lastError", "taskID")
	if err != nil || !ok {
		return nil
	}

	return &s
}

// GetCodes implements LastError.
func (u unstructuredLastErrorAccessor) GetCodes() []gardencorev1beta1.ErrorCode {
	codeStrings := nestedStringSlice(u.Object, "status", "lastError", "codes")
	var codes []gardencorev1beta1.ErrorCode
	for _, codeString := range codeStrings {
		codes = append(codes, gardencorev1beta1.ErrorCode(codeString))
	}
	return codes
}

// GetLastUpdateTime implements LastError.
func (u unstructuredLastErrorAccessor) GetLastUpdateTime() *metav1.Time {
	s, ok, err := unstructured.NestedString(u.Object, "status", "lastError", "lastUpdateTime")
	if err != nil || !ok {
		return nil
	}

	var timestamp metav1.Time
	if err := timestamp.UnmarshalQueryParameter(s); err != nil {
		return nil
	}
	return &timestamp
}

// GetLastError implements Status.
func (u unstructuredStatusAccessor) GetLastError() extensionsv1alpha1.LastError {
	if _, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "lastError"); err != nil || !ok {
		return nil
	}
	return unstructuredLastErrorAccessor(u)
}

// GetExtensionStatus implements Object.
func (u unstructuredAccessor) GetExtensionStatus() extensionsv1alpha1.Status {
	return unstructuredStatusAccessor(u)
}

// GetExtensionSpec implements Object.
func (u unstructuredAccessor) GetExtensionSpec() extensionsv1alpha1.Spec {
	return unstructuredSpecAccessor(u)
}
