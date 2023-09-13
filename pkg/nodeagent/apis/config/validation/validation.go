// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	"github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
)

// ValidateNodeAgentConfiguration validates the given `NodeAgentConfiguration`.
func ValidateNodeAgentConfiguration(conf *config.NodeAgentConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("nodeAgent")

	configFldPath := fldPath.Child("config")

	if conf.HyperkubeImage == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("hyperkubeImage"), "must provide a hyperkubeImage"))
	}
	if conf.Image == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("image"), "must provide a image"))
	}

	if conf.KubernetesVersion == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("kubernetesVersion"), "must provide a supported kubernetesVersion"))
	} else if err := kubernetesversion.CheckIfSupported(conf.KubernetesVersion); err != nil {
		allErrs = append(allErrs, field.Invalid(configFldPath.Child("kubernetesVersion"), conf.KubernetesVersion, err.Error()))
	}

	if conf.OperatingSystemConfigSecretName == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("oscSecretName"), "must provide a oscSecretName"))
	}
	if conf.AccessTokenSecretName == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("tokenSecretName"), "must provide a tokenSecretName"))
	}

	apiServerFldPath := configFldPath.Child("apiServer")

	if conf.APIServer.URL == "" {
		allErrs = append(allErrs, field.Required(apiServerFldPath.Child("url"), "must provide a url"))
	}
	if len(conf.APIServer.CABundle) == 0 {
		allErrs = append(allErrs, field.Required(apiServerFldPath.Child("ca"), "must provide a ca"))
	}
	if conf.APIServer.BootstrapToken == "" {
		allErrs = append(allErrs, field.Required(apiServerFldPath.Child("bootstrapToken"), "must provide a bootstrapToken"))
	}
	return allErrs
}
