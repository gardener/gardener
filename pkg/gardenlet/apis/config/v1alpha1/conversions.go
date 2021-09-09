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
	"k8s.io/apimachinery/pkg/conversion"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func Convert_v1beta1_SeedTemplate_To_core_SeedTemplate(in *gardencorev1beta1.SeedTemplate, out *gardencore.SeedTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_v1beta1_SeedTemplate_To_core_SeedTemplate(in, out, s)
}

func Convert_core_SeedTemplate_To_v1beta1_SeedTemplate(in *gardencore.SeedTemplate, out *gardencorev1beta1.SeedTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_core_SeedTemplate_To_v1beta1_SeedTemplate(in, out, s)
}
