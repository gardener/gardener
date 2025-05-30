//  SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// ExtensionRequirementsChanged is a predicate which returns 'true' if any required condition changed for the extension object.
func ExtensionRequirementsChanged() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			ext, ok := e.ObjectNew.(*operatorv1alpha1.Extension)
			if !ok {
				return false
			}

			oldExt, ok := e.ObjectOld.(*operatorv1alpha1.Extension)
			if !ok {
				return false
			}

			requiredRuntimeCondition := v1beta1helper.GetCondition(ext.Status.Conditions, operatorv1alpha1.ExtensionRequiredRuntime)
			oldRequiredRuntimeCondition := v1beta1helper.GetCondition(oldExt.Status.Conditions, operatorv1alpha1.ExtensionRequiredRuntime)

			if (oldRequiredRuntimeCondition == nil && requiredRuntimeCondition != nil) ||
				(oldRequiredRuntimeCondition != nil && requiredRuntimeCondition == nil) {
				return true
			}
			if oldRequiredRuntimeCondition != nil && requiredRuntimeCondition != nil {
				return oldRequiredRuntimeCondition.Status != requiredRuntimeCondition.Status
			}

			return false
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
