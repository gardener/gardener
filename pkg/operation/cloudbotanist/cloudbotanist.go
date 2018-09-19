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

package cloudbotanist

import (
	"errors"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/alicloudbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/awsbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/azurebotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/gcpbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/localbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/openstackbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
)

// New creates a Cloud Botanist for the specific cloud provider of the operation.
// The Cloud Botanist is responsible for all operations which require IaaS specific knowledge.
// We store the infrastructure credentials on the Botanist object for later usage so that we do not
// need to read the IaaS Secret again.
func New(o *operation.Operation, purpose string) (CloudBotanist, error) {
	var cloudProvider gardenv1beta1.CloudProvider
	switch purpose {
	case common.CloudPurposeShoot:
		cloudProvider = o.Shoot.CloudProvider
	case common.CloudPurposeSeed:
		cloudProvider = o.Seed.CloudProvider
	default:
		return nil, errors.New("unsupported cloud botanist purpose")
	}

	switch cloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		return awsbotanist.New(o, purpose)
	case gardenv1beta1.CloudProviderAzure:
		return azurebotanist.New(o, purpose)
	case gardenv1beta1.CloudProviderGCP:
		return gcpbotanist.New(o, purpose)
	case gardenv1beta1.CloudProviderAlicloud:
		return alicloudbotanist.New(o, purpose)
	case gardenv1beta1.CloudProviderOpenStack:
		return openstackbotanist.New(o, purpose)
	case gardenv1beta1.CloudProviderLocal:
		return localbotanist.New(o)
	default:
		return nil, errors.New("unsupported cloud provider")
	}
}
