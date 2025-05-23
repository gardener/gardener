// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
func (ExposureClassStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {
}

// PrepareForUpdate allows to modify an object before it get stored.
// It is called before ValidateUpdate.
func (ExposureClassStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {
}

// Validate allow to validate the object.
func (ExposureClassStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	exposureClass := obj.(*core.ExposureClass)
	return validation.ValidateExposureClass(exposureClass)
}

// ValidateUpdate validates the update on the object.
// The old and the new version of the object are passed in.
func (ExposureClassStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldExposureClass, newExposureClass := oldObj.(*core.ExposureClass), newObj.(*core.ExposureClass)
	return validation.ValidateExposureClassUpdate(newExposureClass, oldExposureClass)
}

// Canonicalize can be used to transform the object into its canonical format.
func (ExposureClassStrategy) Canonicalize(_ runtime.Object) {
}

// AllowCreateOnUpdate indicates if the object can be created via a PUT operation.
func (ExposureClassStrategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate indicates if the object can be updated
// independently of the resource version.
func (ExposureClassStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (ExposureClassStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (ExposureClassStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
