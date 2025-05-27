// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/api/extensions"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
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

// AddTypeAndClassPredicates returns a new slice which contains a HasClass, a type predicate and the given `predicates`.
// If more than one extensionTypes is given they are combined with an OR.
func AddTypeAndClassPredicates(predicates []predicate.Predicate, extensionClass extensionsv1alpha1.ExtensionClass, extensionTypes ...string) []predicate.Predicate {
	resultPredicates := make([]predicate.Predicate, 0, len(predicates)+2)
	resultPredicates = append(resultPredicates, HasClass(extensionClass))
	resultPredicates = append(resultPredicates, HasOneOfTypesPredicate(extensionTypes...))
	resultPredicates = append(resultPredicates, predicates...)
	return resultPredicates
}

// HasOneOfTypesPredicate returns a new slice which contains a type predicate.
// If more than one extensionTypes is given they are combined with an OR.
func HasOneOfTypesPredicate(extensionTypes ...string) predicate.Predicate {
	if len(extensionTypes) == 1 {
		return HasType(extensionTypes[0])
	}

	orPreds := make([]predicate.Predicate, 0, len(extensionTypes))
	for _, extensionType := range extensionTypes {
		orPreds = append(orPreds, HasType(extensionType))
	}

	return predicate.Or(orPreds...)
}

// HasClass filters the incoming objects for the given extension classes.
// For backwards compatibility, if the class is unset in the extension object, it is assumed that the extension belongs to a shoot cluster.
// An empty 'extensionClass' is likewise treated to be of class 'shoot'.
func HasClass(extensionClasses ...extensionsv1alpha1.ExtensionClass) predicate.Predicate {
	if len(extensionClasses) == 0 {
		extensionClasses = append(extensionClasses, extensionsv1alpha1.ExtensionClassShoot)
	} else if len(extensionClasses) == 1 && extensionClasses[0] == "" {
		extensionClasses[0] = extensionsv1alpha1.ExtensionClassShoot
	}

	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj == nil {
			return false
		}

		accessor, err := extensions.Accessor(obj)
		if err != nil {
			return false
		}

		return set.New(extensionClasses...).Has(extensionsv1alpha1helper.GetExtensionClassOrDefault(accessor.GetExtensionSpec().GetExtensionClass()))
	})
}

// HasPurpose filters the incoming ControlPlanes for the given spec.purpose.
//
// Deprecated: Purpose field is being deprecated and will be removed in gardener v1.123.0.
// The value "exposure" is no longer used since the enablement of SNI, and the value "normal" is redundant.
// TODO(theoddora): Remove this function in v1.123.0 when the Purpose field is removed.
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
