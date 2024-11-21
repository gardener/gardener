// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/api/extensions"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var logger = log.Log.WithName("predicate")

// HasType filters the incoming OperatingSystemConfigs for ones that have the same type
// as the given type.
func HasType(typeName string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		acc, err := extensions.Accessor(obj)
		if err != nil {
			return false
		}

		return acc.GetExtensionSpec().GetExtensionType() == typeName
	})
}

// AddTypePredicate returns a new slice which contains a type predicate and the given `predicates`.
// if more than one extensionTypes is given all given types are or combined
func AddTypePredicate(predicates []predicate.Predicate, extensionTypes ...string) []predicate.Predicate {
	resultPredicates := make([]predicate.Predicate, 0, len(predicates)+1)
	resultPredicates = append(resultPredicates, predicates...)

	if len(extensionTypes) == 1 {
		resultPredicates = append(resultPredicates, HasType(extensionTypes[0]))
		return resultPredicates
	}

	orPreds := make([]predicate.Predicate, 0, len(extensionTypes))
	for _, extensionType := range extensionTypes {
		orPreds = append(orPreds, HasType(extensionType))
	}

	return append(resultPredicates, predicate.Or(orPreds...))
}

// HasClass filters the incoming objects for the given extension class.
// For backwards compatibility, if the extension class is unset, it is assumed that the extension belongs to a shoot cluster.
// An empty given 'extensionClass' is likewise treated to be of class 'shoot'.
func HasClass(extensionClass extensionsv1alpha1.ExtensionClass) predicate.Predicate {
	if extensionClass == "" {
		extensionClass = extensionsv1alpha1.ExtensionClassShoot
	}

	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj == nil {
			return false
		}

		accessor, err := extensions.Accessor(obj)
		if err != nil {
			return false
		}

		return ptr.Deref(accessor.GetExtensionSpec().GetExtensionClass(), extensionsv1alpha1.ExtensionClassShoot) == extensionClass
	})
}

// HasPurpose filters the incoming ControlPlanes for the given spec.purpose.
func HasPurpose(purpose extensionsv1alpha1.Purpose) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		controlPlane, ok := obj.(*extensionsv1alpha1.ControlPlane)
		if !ok {
			return false
		}

		// needed because ControlPlane of type "normal" has the spec.purpose field not set
		if controlPlane.Spec.Purpose == nil && purpose == extensionsv1alpha1.Normal {
			return true
		}

		if controlPlane.Spec.Purpose == nil {
			return false
		}

		return *controlPlane.Spec.Purpose == purpose
	})
}
