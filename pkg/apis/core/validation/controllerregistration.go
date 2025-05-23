// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"
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
	core.ControllerDeploymentPolicyOnDemand,
	core.ControllerDeploymentPolicyAlways,
	core.ControllerDeploymentPolicyAlwaysExceptNoShoots,
)

var availableExtensionStrategies = sets.New(
	core.BeforeKubeAPIServer,
	core.AfterKubeAPIServer,
)

var availableExtensionStrategiesForReconcile = availableExtensionStrategies.Clone().Insert(
	core.AfterWorker,
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

	allErrs = append(allErrs, ValidateControllerResources(spec.Resources, []core.ClusterType{core.ClusterTypeShoot, core.ClusterTypeSeed}, fldPath.Child("resources"))...)

	if deployment := spec.Deployment; deployment != nil {
		deploymentPath := fldPath.Child("deployment")

		if policy := deployment.Policy; policy != nil && !availablePolicies.Has(*policy) {
			allErrs = append(allErrs, field.NotSupported(deploymentPath.Child("policy"), *policy, sets.List(availablePolicies)))
		}

		if deployment.SeedSelector != nil {
			controlsResourcesPrimarily := slices.ContainsFunc(spec.Resources, func(resource core.ControllerResource) bool {
				return resource.Primary == nil || *resource.Primary
			})

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

// ValidateControllerResources validates the provided list of ControllerResource objects.
func ValidateControllerResources(resources []core.ControllerResource, clusterTypes []core.ClusterType, resourcesPath *field.Path) field.ErrorList {
	var (
		allErrs            = field.ErrorList{}
		resourceKindToType = make(map[string]string)
	)

	for i, resource := range resources {
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

		if t, ok := resourceKindToType[resource.Kind]; ok && t == resource.Type {
			allErrs = append(allErrs, field.Duplicate(idxPath, gardenerutils.ExtensionsID(resource.Kind, resource.Type)))
		}
		resourceKindToType[resource.Kind] = resource.Type

		if resource.Kind != extensionsv1alpha1.ExtensionResource {
			if resource.GloballyEnabled != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("globallyEnabled"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
			if len(resource.AutoEnable) > 0 {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("autoEnable"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
			if len(resource.ClusterCompatibility) > 0 {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("clusterCompatibility"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
			if resource.ReconcileTimeout != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("reconcileTimeout"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
			if resource.Lifecycle != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("lifecycle"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}

			continue
		}

		var (
			validClusterTypes      = sets.New(clusterTypes...)
			compatibleClusterTypes = sets.New[core.ClusterType]()
		)

		for j, clusterType := range resource.ClusterCompatibility {
			autoEnablePath := idxPath.Child("clusterCompatibility").Index(j)

			if !validClusterTypes.Has(clusterType) {
				allErrs = append(allErrs, field.NotSupported(autoEnablePath, clusterType, sets.List(validClusterTypes)))
			}

			if compatibleClusterTypes.Has(clusterType) {
				allErrs = append(allErrs, field.Duplicate(autoEnablePath, clusterType))
			}
			compatibleClusterTypes.Insert(clusterType)
		}

		autoEnabledClusterTypes := sets.New[core.ClusterType]()
		for j, clusterType := range resource.AutoEnable {
			autoEnablePath := idxPath.Child("autoEnable").Index(j)

			if !validClusterTypes.Has(clusterType) {
				allErrs = append(allErrs, field.NotSupported(autoEnablePath, clusterType, sets.List(validClusterTypes)))
			}

			if !compatibleClusterTypes.Has(clusterType) {
				allErrs = append(allErrs, field.Forbidden(autoEnablePath, fmt.Sprintf("autoEnable is not allowed for cluster type %q when clusterCompatibility is set to %+v", clusterType, compatibleClusterTypes.UnsortedList())))
			}

			if autoEnabledClusterTypes.Has(clusterType) {
				allErrs = append(allErrs, field.Duplicate(autoEnablePath, clusterType))
			}
			autoEnabledClusterTypes.Insert(clusterType)
		}

		if resource.Lifecycle != nil {
			lifecyclePath := idxPath.Child("lifecycle")
			if resource.Lifecycle.Reconcile != nil && !availableExtensionStrategiesForReconcile.Has(*resource.Lifecycle.Reconcile) {
				allErrs = append(allErrs, field.NotSupported(lifecyclePath.Child("reconcile"), *resource.Lifecycle.Reconcile, sets.List(availableExtensionStrategiesForReconcile)))
			}
			if resource.Lifecycle.Delete != nil && !availableExtensionStrategies.Has(*resource.Lifecycle.Delete) {
				allErrs = append(allErrs, field.NotSupported(lifecyclePath.Child("delete"), *resource.Lifecycle.Delete, sets.List(availableExtensionStrategies)))
			}
			if resource.Lifecycle.Migrate != nil && !availableExtensionStrategies.Has(*resource.Lifecycle.Migrate) {
				allErrs = append(allErrs, field.NotSupported(lifecyclePath.Child("migrate"), *resource.Lifecycle.Migrate, sets.List(availableExtensionStrategies)))
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
		if diff := deep.Equal(new, old); diff != nil {
			return field.ErrorList{field.Forbidden(fldPath, strings.Join(diff, ","))}
		}
		return apivalidation.ValidateImmutableField(new, old, fldPath)
	}

	allErrs = append(allErrs, ValidateControllerResourcesUpdate(new.Resources, old.Resources, fldPath.Child("resources"))...)

	return allErrs
}

// ValidateControllerResourcesUpdate validates the update of ControllerResource objects.
func ValidateControllerResourcesUpdate(new, old []core.ControllerResource, fldPath *field.Path) field.ErrorList {
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
