// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
)

// ValidateNodeAgentConfiguration validates the given `NodeAgentConfiguration`.
func ValidateNodeAgentConfiguration(conf *config.NodeAgentConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("nodeagent")

	configFldPath := fldPath.Child("config")

	if conf.APIVersion == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("apiversion"), "must provide a apiversion"))
	}
	if conf.HyperkubeImage == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("hyperkubeimage"), "must provide a hyperkubeimage"))
	}
	if conf.Image == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("image"), "must provide a image"))
	}
	if conf.KubernetesVersion == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("kubernetesversion"), "must provide a kubernetesversion"))
	}
	if conf.OSCSecretName == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("oscsecretname"), "must provide a oscsecretname"))
	}
	if conf.TokenSecretName == "" {
		allErrs = append(allErrs, field.Required(configFldPath.Child("tokensecretname"), "must provide a tokensecretname"))
	}

	apiserverFldPath := configFldPath.Child("apiserver")

	if conf.APIServer.URL == "" {
		allErrs = append(allErrs, field.Required(apiserverFldPath.Child("url"), "must provide a url"))
	}
	if conf.APIServer.CA == "" {
		allErrs = append(allErrs, field.Required(apiserverFldPath.Child("ca"), "must provide a ca"))
	}
	if conf.APIServer.BootstrapToken == "" {
		allErrs = append(allErrs, field.Required(apiserverFldPath.Child("bootstraptoken"), "must provide a bootstraptoken"))
	}
	return allErrs
}
