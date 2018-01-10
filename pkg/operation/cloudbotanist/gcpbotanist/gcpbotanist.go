// Copyright 2018 The Gardener Authors.
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

package gcpbotanist

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/utils"
)

// New takes an operation object <o> and creates a new GCPBotanist object.
func New(o *operation.Operation) (*GCPBotanist, error) {
	if o.Shoot.Info.Spec.Cloud.GCP == nil {
		return nil, errors.New("cannot instantiate an GCP botanist if `.spec.cloud.gcp` is nil")
	}

	vpcName := ""
	if vpc := o.Shoot.Info.Spec.Cloud.GCP.Networks.VPC; vpc != nil {
		vpcName = vpc.Name
	}

	// Read project id out of the service account
	serviceAccount := utils.ConvertJSONToMap(o.Shoot.Secret.Data[ServiceAccountJSON])
	project, err := serviceAccount.String(ProjectID)
	if err != nil {
		return nil, err
	}

	// Minify serviceaccount json to allow injection into Terraform environment
	buf := new(bytes.Buffer)
	err = json.Compact(buf, o.Shoot.Secret.Data[ServiceAccountJSON])
	if err != nil {
		return nil, err
	}

	return &GCPBotanist{
		Operation:              o,
		CloudProviderName:      "gce",
		VPCName:                vpcName,
		Project:                project,
		MinifiedServiceAccount: buf.String(),
	}, nil
}

// GetCloudProviderName returns the Kubernetes cloud provider name for this cloud.
func (b *GCPBotanist) GetCloudProviderName() string {
	return b.CloudProviderName
}
