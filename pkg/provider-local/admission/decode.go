// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"k8s.io/apimachinery/pkg/runtime"

	localv1alpha1 "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
)

// DecodeCloudProfileConfig decodes the given RawExtension into a CloudProfileConfig.
func DecodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*localv1alpha1.CloudProfileConfig, error) {
	cloudProfileConfig := &localv1alpha1.CloudProfileConfig{}
	if err := runtime.DecodeInto(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}
