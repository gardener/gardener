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

package gardener

import (
	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// GetMandatoryExposureClassHandlerSNILabels get the labels of an ExposureClass Handler plus its name
// and will add the mandatory SNI labels for ExposureClass handlers to it.
// Existing label keys will be overridden by the mandatory labels keys.
func GetMandatoryExposureClassHandlerSNILabels(labels map[string]string, exposureClassName string) map[string]string {
	return utils.MergeStringMaps(labels, map[string]string{
		v1beta1constants.LabelApp:                       gardenletconfigv1alpha1.DefaultIngressGatewayAppLabelValue,
		v1alpha1constants.GardenRole:                    v1alpha1constants.GardenRoleExposureClassHandler,
		v1alpha1constants.LabelExposureClassHandlerName: exposureClassName,
	})
}
