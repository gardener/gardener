// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// Handler performs validation.
type Handler struct{}

// ValidateDelete performs the validation.
func (h *Handler) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	extension, ok := obj.(*operatorv1alpha1.Extension)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Extension but got %T", obj)
	}

	for _, conditionType := range []gardencorev1beta1.ConditionType{
		operatorv1alpha1.ExtensionRequiredRuntime,
		operatorv1alpha1.ExtensionRequiredVirtual,
	} {
		requiredCondition := v1beta1helper.GetCondition(extension.Status.Conditions, conditionType)
		if requiredCondition != nil && requiredCondition.Status == gardencorev1beta1.ConditionTrue {
			return nil, fmt.Errorf("extension is still being required: %q", requiredCondition.Message)
		}
	}

	return nil, nil
}

// ValidateCreate performs the validation.
func (h *Handler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate performs the validation.
func (h *Handler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
