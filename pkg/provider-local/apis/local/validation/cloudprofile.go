// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cloudProfile *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImagesPath := fldPath.Child("machineImages")
	if len(cloudProfile.MachineImages) == 0 {
		allErrs = append(allErrs, field.Required(machineImagesPath, "must provide at least one machine image"))
	}

	for i, machineImage := range cloudProfile.MachineImages {
		idxPath := machineImagesPath.Index(i)

		if len(machineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}

		if len(machineImage.Versions) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", machineImage.Name)))
		}
		for j, version := range machineImage.Versions {
			jdxPath := idxPath.Child("versions").Index(j)

			if len(version.Version) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
			}

			if len(version.Image) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("image"), "must provide an image"))
			}
		}
	}

	return allErrs
}
