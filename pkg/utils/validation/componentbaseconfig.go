// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	componentbaseconfigvalidation "k8s.io/component-base/config/validation"
)

var configScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(componentbaseconfigv1alpha1.AddToScheme(configScheme))
}

// ValidateClientConnectionConfiguration validates a componentbaseconfigv1alpha1.ClientConnectionConfiguration by
// converting it to the internal version and calling the validation from the k8s.io/component-base/config/validation
// package.
func ValidateClientConnectionConfiguration(config *componentbaseconfigv1alpha1.ClientConnectionConfiguration, fldPath *field.Path) field.ErrorList {
	if config == nil {
		return nil
	}

	var allErrs field.ErrorList

	internalClientConnectionConfig := &componentbaseconfig.ClientConnectionConfiguration{}
	if err := configScheme.Convert(config, internalClientConnectionConfig, nil); err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
	} else {
		allErrs = append(allErrs, componentbaseconfigvalidation.ValidateClientConnectionConfiguration(internalClientConnectionConfig, fldPath)...)
	}

	return allErrs
}

// ValidateLeaderElectionConfiguration validates a componentbaseconfigv1alpha1.LeaderElectionConfiguration by
// converting it to the internal version and calling the validation from the k8s.io/component-base/config/validation
// package.
func ValidateLeaderElectionConfiguration(config *componentbaseconfigv1alpha1.LeaderElectionConfiguration, fldPath *field.Path) field.ErrorList {
	if config == nil {
		return nil
	}

	var allErrs field.ErrorList

	internalLeaderElectionConfig := &componentbaseconfig.LeaderElectionConfiguration{}
	if err := configScheme.Convert(config, internalLeaderElectionConfig, nil); err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
	} else {
		allErrs = append(allErrs, componentbaseconfigvalidation.ValidateLeaderElectionConfiguration(internalLeaderElectionConfig, fldPath)...)
	}

	return allErrs
}
