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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

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

func nestedInt64(obj map[string]interface{}, fields ...string) int64 {
	v, ok, err := unstructured.NestedInt64(obj, fields...)
	if err != nil || !ok {
		return 0
	}
	return v
}

func nestedInt(obj map[string]interface{}, fields ...string) int {
	v, ok, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !ok {
		return 0
	}

	switch x := v.(type) {
	case int64:
		return int(x)
	case int32:
		return int(x)
	case int:
		return x
	default:
		return 0
	}
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
func (u unstructuredLastOperationAccessor) GetProgress() int {
	return nestedInt(u.UnstructuredContent(), "status", "lastOperation", "progress")
}

// GetState implements LastOperation.
func (u unstructuredLastOperationAccessor) GetState() gardencorev1alpha1.LastOperationState {
	return gardencorev1alpha1.LastOperationState(nestedString(u.UnstructuredContent(), "status", "lastOperation", "state"))
}

// GetType implements LastOperation.
func (u unstructuredLastOperationAccessor) GetType() gardencorev1alpha1.LastOperationType {
	return gardencorev1alpha1.LastOperationType(nestedString(u.UnstructuredContent(), "status", "lastOperation", "type"))
}

// GetExtensionType implements Spec.
func (u unstructuredSpecAccessor) GetExtensionType() string {
	return nestedString(u.UnstructuredContent(), "spec", "type")
}

// GetLastOperation implements Status.
func (u unstructuredStatusAccessor) GetLastOperation() extensionsv1alpha1.LastOperation {
	if _, ok, err := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), "status", "lastOperation"); err != nil || !ok {
		return nil
	}
	return unstructuredLastOperationAccessor{u.Unstructured}
}

// GetObservedGeneration implements Status.
func (u unstructuredStatusAccessor) GetObservedGeneration() int64 {
	return nestedInt64(u.Object, "status", "observedGeneration")
}

// GetDescription implements LastError.
func (u unstructuredLastErrorAccessor) GetDescription() string {
	return nestedString(u.Object, "status", "lastError", "description")
}

// GetCodes implements LastError.
func (u unstructuredLastErrorAccessor) GetCodes() []gardencorev1alpha1.ErrorCode {
	codeStrings := nestedStringSlice(u.Object, "status", "lastError", "codes")
	var codes []gardencorev1alpha1.ErrorCode
	for _, codeString := range codeStrings {
		codes = append(codes, gardencorev1alpha1.ErrorCode(codeString))
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
	return unstructuredLastErrorAccessor{u.Unstructured}
}

// GetExtensionStatus implements Object.
func (u unstructuredAccessor) GetExtensionStatus() extensionsv1alpha1.Status {
	return unstructuredStatusAccessor{u.Unstructured}
}

// GetExtensionSpec implements Object.
func (u unstructuredAccessor) GetExtensionSpec() extensionsv1alpha1.Spec {
	return unstructuredSpecAccessor{u.Unstructured}
}
