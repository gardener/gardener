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

package validation

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var availablePolicies = sets.NewString(
	string(core.ControllerDeploymentPolicyOnDemand),
	string(core.ControllerDeploymentPolicyAlways),
	string(core.ControllerDeploymentPolicyAlwaysExceptNoShoots),
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

		if !extensionsv1alpha1.ExtensionKinds.Has(resource.Kind) {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("kind"), resource.Kind, extensionsv1alpha1.ExtensionKinds.UnsortedList()))
		}

		if len(resource.Type) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("type"), "field is required"))
		}
		if t, ok := resources[resource.Kind]; ok && t == resource.Type {
			allErrs = append(allErrs, field.Duplicate(idxPath, extensions.Id(resource.Kind, resource.Type)))
		}
		if resource.Kind != extensionsv1alpha1.ExtensionResource {
			if resource.GloballyEnabled != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("globallyEnabled"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
			if resource.ReconcileTimeout != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("reconcileTimeout"), fmt.Sprintf("field must not be set when kind != %s", extensionsv1alpha1.ExtensionResource)))
			}
		}

		resources[resource.Kind] = resource.Type
		if resource.Primary == nil || *resource.Primary {
			controlsResourcesPrimarily = true
		}
	}

	if deployment := spec.Deployment; deployment != nil {
		if policy := deployment.Policy; policy != nil && !availablePolicies.Has(string(*policy)) {
			allErrs = append(allErrs, field.NotSupported(deploymentPath.Child("policy"), *policy, availablePolicies.List()))
		}

		if deployment.SeedSelector != nil {
			if controlsResourcesPrimarily {
				allErrs = append(allErrs, field.Forbidden(deploymentPath.Child("seedSelector"), "specifying a seed selector is not allowed when controlling resources primarily"))
			}

			allErrs = append(allErrs, metav1validation.ValidateLabelSelector(deployment.SeedSelector, deploymentPath.Child("seedSelector"))...)
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
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	kindTypeToPrimary := make(map[string]*bool, len(old.Resources))
	for _, resource := range old.Resources {
		kindTypeToPrimary[resource.Kind+resource.Type] = resource.Primary
	}
	for i, resource := range new.Resources {
		if primary, ok := kindTypeToPrimary[resource.Kind+resource.Type]; ok {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(resource.Primary, primary, fldPath.Child("resources").Index(i).Child("primary"))...)
		}
	}

	return allErrs
}
