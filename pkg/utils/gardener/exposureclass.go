// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

// GetMandatoryExposureClassHandlerSNILabels get the labels of an ExposureClass Handler plus its name
// and will add the mandatory SNI labels for ExposureClass handlers to it.
// Existing label keys will be overridden by the mandatory labels keys.
func GetMandatoryExposureClassHandlerSNILabels(labels map[string]string, exposureClassName string) map[string]string {
	return utils.MergeStringMaps(labels, map[string]string{
		v1beta1constants.LabelApp:                      v1beta1constants.DefaultIngressGatewayAppLabelValue,
		v1beta1constants.GardenRole:                    v1beta1constants.GardenRoleExposureClassHandler,
		v1beta1constants.LabelExposureClassHandlerName: exposureClassName,
	})
}
