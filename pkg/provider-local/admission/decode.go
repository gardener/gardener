// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/extensions/pkg/util"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
)

// DecodeCloudProfileConfig decodes the given RawExtension into a CloudProfileConfig.
func DecodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*api.CloudProfileConfig, error) {
	cloudProfileConfig := &api.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}
