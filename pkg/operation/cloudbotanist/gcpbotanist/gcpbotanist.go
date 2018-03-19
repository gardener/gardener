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

package gcpbotanist

import (
	"bytes"
	"encoding/json"
	"errors"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
)

// New takes an operation object <o> and creates a new GCPBotanist object.
func New(o *operation.Operation, purpose string) (*GCPBotanist, error) {
	var cloudProvider gardenv1beta1.CloudProvider
	switch purpose {
	case common.CloudPurposeShoot:
		cloudProvider = o.Shoot.CloudProvider
	case common.CloudPurposeSeed:
		cloudProvider = o.Seed.CloudProvider
	}

	if cloudProvider != gardenv1beta1.CloudProviderGCP {
		return nil, errors.New("cannot instantiate an GCP botanist if neither Shoot nor Seed cluster specifies GCP")
	}

	// Read vpc name out of the Shoot manifest
	vpcName := ""
	if purpose == common.CloudPurposeShoot {
		if gcp := o.Shoot.Info.Spec.Cloud.GCP; gcp != nil {
			if vpc := gcp.Networks.VPC; vpc != nil {
				vpcName = vpc.Name
			}
		}
	}

	// Read project id out of the service account
	var serviceAccountJSON []byte
	switch purpose {
	case common.CloudPurposeShoot:
		serviceAccountJSON = o.Shoot.Secret.Data[ServiceAccountJSON]
	case common.CloudPurposeSeed:
		serviceAccountJSON = o.Seed.Secret.Data[ServiceAccountJSON]
	}

	project, err := ExtractProjectID(serviceAccountJSON)
	if err != nil {
		return nil, err
	}

	// Minify serviceaccount json to allow injection into Terraform environment
	minifiedServiceAccount, err := MinifyServiceAccount(serviceAccountJSON)
	if err != nil {
		return nil, err
	}

	return &GCPBotanist{
		Operation:              o,
		CloudProviderName:      "gce",
		VPCName:                vpcName,
		Project:                project,
		MinifiedServiceAccount: minifiedServiceAccount,
	}, nil
}

// GetCloudProviderName returns the Kubernetes cloud provider name for this cloud.
func (b *GCPBotanist) GetCloudProviderName() string {
	return b.CloudProviderName
}

// MinifyServiceAccount uses the provided service account JSON objects and minifies it.
// This is required when you want to inject it as environment variable into Terraform.
func MinifyServiceAccount(serviceAccountJSON []byte) (string, error) {
	buf := new(bytes.Buffer)
	if err := json.Compact(buf, serviceAccountJSON); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ExtractProjectID extracts the value of the key "project_id" from the service account
// JSON document.
func ExtractProjectID(ServiceAccountJSON []byte) (string, error) {
	var j struct {
		Project string `json:"project_id"`
	}
	if err := json.Unmarshal(ServiceAccountJSON, &j); err != nil {
		return "Error", err
	}
	return j.Project, nil
}
