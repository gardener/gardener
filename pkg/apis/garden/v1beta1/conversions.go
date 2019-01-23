// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	"k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(addConversionFuncs)
}

func Convert_v1beta1_Worker_To_garden_Worker(in *Worker, out *garden.Worker, s conversion.Scope) error {
	autoConvert_v1beta1_Worker_To_garden_Worker(in, out, s)
	if in.MaxSurge == nil {
		out.MaxSurge = DefaultWorkerMaxSurge
	} else {
		out.MaxSurge = *in.MaxSurge
	}
	if in.MaxUnavailable == nil {
		out.MaxUnavailable = DefaultWorkerMaxUnavailable
	} else {
		out.MaxUnavailable = *in.MaxUnavailable
	}
	return nil
}

func Convert_garden_Worker_To_v1beta1_Worker(in *garden.Worker, out *Worker, s conversion.Scope) error {
	autoConvert_garden_Worker_To_v1beta1_Worker(in, out, s)
	out.MaxSurge = &in.MaxSurge
	out.MaxUnavailable = &in.MaxUnavailable
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {

	// Add field conversion funcs.
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				garden.ShootSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	)
}
