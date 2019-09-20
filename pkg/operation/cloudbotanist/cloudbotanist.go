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

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/alicloudbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/awsbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/azurebotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/gcpbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/openstackbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/packetbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
)

// New creates a Cloud Botanist for the specific cloud provider of the operation.
// The Cloud Botanist is responsible for all operations which require IaaS specific knowledge.
// We store the infrastructure credentials on the Botanist object for later usage so that we do not
// need to read the IaaS Secret again.
func New(o *operation.Operation, purpose string) (CloudBotanist, error) {
	var cloudProvider string

	switch purpose {
	case common.CloudPurposeShoot:
		cloudProvider = o.Shoot.Info.Spec.Provider.Type
	case common.CloudPurposeSeed:
		cloudProvider = o.Seed.Info.Spec.Provider.Type
	default:
		return nil, errors.New("unsupported cloud botanist purpose")
	}

	switch cloudProvider {
	case "aws":
		return awsbotanist.New(o, purpose)
	case "azure":
		return azurebotanist.New(o, purpose)
	case "gcp":
		return gcpbotanist.New(o, purpose)
	case "alicloud":
		return alicloudbotanist.New(o, purpose)
	case "openstack":
		return openstackbotanist.New(o, purpose)
	case "packet":
		return packetbotanist.New(o, purpose)
	default:
		return nil, errors.New("unsupported cloud provider")
	}
}
