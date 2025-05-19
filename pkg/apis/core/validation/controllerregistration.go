// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/go-test/deep"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var availablePolicies = sets.New(
	string(core.ControllerDeploymentPolicyOnDemand),
	string(core.ControllerDeploymentPolicyAlways),
	string(core.ControllerDeploymentPolicyAlwaysExceptNoShoots),
)

var availableExtensionStrategies = sets.New(
	string(core.BeforeKubeAPIServer),
	string(core.AfterKubeAPIServer),
)

var availableExtensionStrategiesForReconcile = availableExtensionStrategies.Clone().Insert(
	string(core.AfterWorker),
)

// ValidateControllerRegistration validates a ControllerRegistration object.
func ValidateControllerRegistration(controllerRegistration *core.ControllerRegistration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&controllerRegistration.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerRegistrationSpec(&controllerRegistration.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateControllerRegistrationSpec validates the specification of a ControllerRegistration object.
func ValidateControllerRegistrationSpec(spec *core.ControllerRegistrationSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var (
		resourcesPath  = fldPath.Child("resources")
		deploymentPath = fldPath.Child("deployment")

		resources                  = make(map[string]string, len(spec.Resources))
		controlsResourcesPrimarily = false
	)

	for i, resource := range spec.Resources {
		idxPath := resourcesPath.Index(i)

		if len(resource.Kind) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("kind"), "field is required"))
		}

		if !extensionsv1alpha1.AllExtensionKinds.Has(resource.Kind) {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("kind"), resource.Kind, extensionsv1alpha1.AllExtensionKinds.UnsortedList()))
		}

		if len(resource.Type) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("type"), "field is required"))
		}
		if t, ok := resources[resource.Kind]; ok && t == resource.Type {
			allErrs = append(allErrs, field.Duplicate(idxPath, gardenerutils.ExtensionsID(resource.Kind, resource.Type)))
		}
		if resource.Kind != extensionsv1alpha1.ExtensionResource {
			if resource.GloballyEnabled != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("globallyEnabled"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
			if resource.ReconcileTimeout != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("reconcileTimeout"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
			if resource.Lifecycle != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("lifecycle"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
		}

		if resource.Kind == extensionsv1alpha1.ExtensionResource && resource.Lifecycle != nil {
			lifecyclePath := idxPath.Child("lifecycle")
			if resource.Lifecycle.Reconcile != nil && !availableExtensionStrategiesForReconcile.Has(string(*resource.Lifecycle.Reconcile)) {
				allErrs = append(allErrs, field.NotSupported(lifecyclePath.Child("reconcile"), *resource.Lifecycle.Reconcile, sets.List(availableExtensionStrategiesForReconcile)))
			}
			if resource.Lifecycle.Delete != nil && !availableExtensionStrategies.Has(string(*resource.Lifecycle.Delete)) {
				allErrs = append(allErrs, field.NotSupported(lifecyclePath.Child("delete"), *resource.Lifecycle.Delete, sets.List(availableExtensionStrategies)))
			}
			if resource.Lifecycle.Migrate != nil && !availableExtensionStrategies.Has(string(*resource.Lifecycle.Migrate)) {
				allErrs = append(allErrs, field.NotSupported(lifecyclePath.Child("migrate"), *resource.Lifecycle.Migrate, sets.List(availableExtensionStrategies)))
			}
		}

		resources[resource.Kind] = resource.Type
		if resource.Primary == nil || *resource.Primary {
			controlsResourcesPrimarily = true
		}
	}

	if deployment := spec.Deployment; deployment != nil {
		if policy := deployment.Policy; policy != nil && !availablePolicies.Has(string(*policy)) {
			allErrs = append(allErrs, field.NotSupported(deploymentPath.Child("policy"), *policy, sets.List(availablePolicies)))
		}

		if deployment.SeedSelector != nil {
			if controlsResourcesPrimarily {
				allErrs = append(allErrs, field.Forbidden(deploymentPath.Child("seedSelector"), "specifying a seed selector is not allowed when controlling resources primarily"))
			}

			allErrs = append(allErrs, metav1validation.ValidateLabelSelector(deployment.SeedSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, deploymentPath.Child("seedSelector"))...)
		}

		deploymentRefsCount := len(deployment.DeploymentRefs)
		if deploymentRefsCount > 1 {
			allErrs = append(allErrs, field.Forbidden(deploymentPath.Child("deploymentRefs"), "only one deployment reference is allowed"))
		}

		for i, deploymentRef := range deployment.DeploymentRefs {
			fld := deploymentPath.Child("deploymentRefs").Index(i)
			if deploymentRef.Name == "" {
				allErrs = append(allErrs, field.Required(fld.Child("name"), "must not be empty"))
			}
		}
	}

	return allErrs
}

// ValidateControllerRegistrationUpdate validates a ControllerRegistration object before an update.
func ValidateControllerRegistrationUpdate(new, old *core.ControllerRegistration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerRegistrationSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateControllerRegistration(new)...)

	return allErrs
}

// ValidateControllerRegistrationSpecUpdate validates a ControllerRegistration spec before an update.
func ValidateControllerRegistrationSpecUpdate(new, old *core.ControllerRegistrationSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		diff := deep.Equal(new, old)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update shoot spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, ValidateControllerResourceUpdate(new.Resources, old.Resources, fldPath.Child("resources"))...)

	return allErrs
}

// ValidateControllerResourceUpdate validates the update of ControllerResource objects.
func ValidateControllerResourceUpdate(new, old []core.ControllerResource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	kindTypeToPrimary := make(map[string]*bool, len(old))
	for _, resource := range old {
		kindTypeToPrimary[gardenerutils.ExtensionsID(resource.Kind, resource.Type)] = resource.Primary
	}
	for i, resource := range new {
		if primary, ok := kindTypeToPrimary[gardenerutils.ExtensionsID(resource.Kind, resource.Type)]; ok {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(resource.Primary, primary, fldPath.Index(i).Child("primary"))...)
		}
	}

	return allErrs
}
