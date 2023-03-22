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

//nolint:revive
package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
)

var quotaDecoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	quotaDecoder = serializer.NewCodecFactory(scheme).UniversalDecoder(corev1.SchemeGroupVersion)
}

func Convert_v1alpha1_QuotaConfiguration_To_config_QuotaConfiguration(in *QuotaConfiguration, out *config.QuotaConfiguration, s conversion.Scope) error {
	err := autoConvert_v1alpha1_QuotaConfiguration_To_config_QuotaConfiguration(in, out, s)
	if err != nil {
		return err
	}

	if out.Config != nil {
		quotaObj, gvk, err := quotaDecoder.Decode(in.Config.Raw, nil, nil)
		if err != nil {
			return err
		}

		quota, ok := quotaObj.(*corev1.ResourceQuota)
		if !ok {
			return fmt.Errorf("%v is not a supported ResourceQuota configuration", gvk)
		}

		out.Config = quota
	}
	return nil
}
