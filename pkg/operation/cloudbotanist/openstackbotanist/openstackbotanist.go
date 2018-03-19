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

package openstackbotanist

import (
	"errors"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
)

// New takes an operation object <o> and creates a new OpenStackBotanist object.
func New(o *operation.Operation, purpose string) (*OpenStackBotanist, error) {
	var cloudProvider gardenv1beta1.CloudProvider
	switch purpose {
	case common.CloudPurposeShoot:
		cloudProvider = o.Shoot.CloudProvider
	case common.CloudPurposeSeed:
		cloudProvider = o.Seed.CloudProvider
	}

	if cloudProvider != gardenv1beta1.CloudProviderOpenStack {
		return nil, errors.New("cannot instantiate an OpenStack botanist if neither Shoot nor Seed cluster specifies OpenStack")
	}

	return &OpenStackBotanist{
		Operation:         o,
		CloudProviderName: "openstack",
	}, nil
}

// GetCloudProviderName returns the Kubernetes cloud provider name for this cloud.
func (b *OpenStackBotanist) GetCloudProviderName() string {
	return b.CloudProviderName
}
