// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/landscaper/pkg/gardenlet/apis/imports"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	"k8s.io/apimachinery/pkg/conversion"
)

func Convert_v1alpha1_Imports_To_imports_Imports(in *Imports, out *imports.Imports, s conversion.Scope) error {
	if in.ComponentConfiguration.Object == nil {
		cfg, err := encoding.DecodeGardenletConfigurationFromBytes(in.ComponentConfiguration.Raw, false)
		if err != nil {
			return err
		}
		in.ComponentConfiguration.Object = cfg
	}
	return autoConvert_v1alpha1_Imports_To_imports_Imports(in, out, s)
}

func Convert_imports_Imports_To_v1alpha1_Imports(in *imports.Imports, out *Imports, s conversion.Scope) error {
	if err := autoConvert_imports_Imports_To_v1alpha1_Imports(in, out, s); err != nil {
		return err
	}
	if out.ComponentConfiguration.Raw == nil {
		cfg, ok := out.ComponentConfiguration.Object.(*configv1alpha1.GardenletConfiguration)
		if !ok {
			return fmt.Errorf("unknown gardenlet config object type")
		}
		raw, err := encoding.EncodeGardenletConfigurationToBytes(cfg)
		if err != nil {
			return err
		}
		out.ComponentConfiguration.Raw = raw
	}
	return nil
}

func Convert_v1alpha1_GardenletDeployment_To_seedmanagement_GardenletDeployment(in *seedmanagementv1alpha1.GardenletDeployment, out *seedmanagement.GardenletDeployment, s conversion.Scope) error {
	return seedmanagementv1alpha1.Convert_v1alpha1_GardenletDeployment_To_seedmanagement_GardenletDeployment(in, out, s)
}

func Convert_seedmanagement_GardenletDeployment_To_v1alpha1_GardenletDeployment(in *seedmanagement.GardenletDeployment, out *seedmanagementv1alpha1.GardenletDeployment, s conversion.Scope) error {
	return seedmanagementv1alpha1.Convert_seedmanagement_GardenletDeployment_To_v1alpha1_GardenletDeployment(in, out, s)
}
