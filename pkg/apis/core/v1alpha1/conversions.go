// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

//nolint:revive
package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/core"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("ControllerInstallation"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", core.RegistrationRefName, core.SeedRefName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", core.ShootSeedName, core.ShootCloudProfileName, core.ShootStatusSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	return nil
}

func removeRoleFromRoles(roles []string, role string) []string {
	var newRoles []string
	for _, r := range roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	return newRoles
}
