// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package exposureclass

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

// ExposureClassStrategy define the strategy for storing exposureclasses.
type ExposureClassStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy return a storage strategy for exposureclasses.
func NewStrategy() ExposureClassStrategy {
	return ExposureClassStrategy{
		api.Scheme,
		names.SimpleNameGenerator,
	}
}

// NamespaceScoped indicates if the object is namespaced scoped.
func (ExposureClassStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate mutates the object before creation.
// It is called before Validate.
func (ExposureClassStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

// PrepareForUpdate allows to modify an object before it get stored.
// It is called before ValidateUpdate.
func (ExposureClassStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
}

// Validate allow to validate the object.
func (ExposureClassStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	exposureClass := obj.(*core.ExposureClass)
	return validation.ValidateExposureClass(exposureClass)
}

// ValidateUpdate validates the update on the object.
// The old and the new version of the object are passed in.
func (ExposureClassStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldExposureClass, newExposureClass := oldObj.(*core.ExposureClass), newObj.(*core.ExposureClass)
	return validation.ValidateExposureClassUpdate(newExposureClass, oldExposureClass)
}

// Canonicalize can be used to transfrom the object into its cannoical format.
func (ExposureClassStrategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate indicates if the object can be created via a PUT operation.
func (ExposureClassStrategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate indicates if the object can be updated
// independently of the resouce version.
func (ExposureClassStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (ExposureClassStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (ExposureClassStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
