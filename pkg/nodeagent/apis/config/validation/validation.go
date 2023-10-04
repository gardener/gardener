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

	if conf.ClientConnection.Kubeconfig == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("clientConnection").Child("kubeconfig"), "must provide a path to a kubeconfig"))
	}

	if conf.HyperkubeImage == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("hyperkubeImage"), "must provide a hyperkube image"))
	}
	if conf.Image == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("image"), "must provide an image"))
	}

	if conf.KubernetesVersion == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("kubernetesVersion"), "must provide a supported kubernetes version"))
	} else if err := kubernetesversion.CheckIfSupported(conf.KubernetesVersion.String()); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("kubernetesVersion"), conf.KubernetesVersion, err.Error()))
	}

	if conf.OperatingSystemConfigSecretName == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("operatingSystemConfigSecretName"), "must provide the secret name for the operating system config"))
	}
	if conf.AccessTokenSecretName == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("accessTokenSecretName"), "must provide the secret name for the access token"))
	}

	return allErrs
}
