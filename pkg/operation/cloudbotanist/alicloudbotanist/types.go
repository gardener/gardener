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

package alicloudbotanist

import (
	"github.com/gardener/gardener/pkg/operation"
)

// AlicloudBotanist is a struct which has methods that perform Alicloud cloud-specific operations for a Shoot cluster.
type AlicloudBotanist struct {
	*operation.Operation
	CloudProviderName string
}

const (
	// AccessKeyID is a constant for the key in a cloud provider secret and backup secret that holds the Alicloud access key id.
	AccessKeyID = "accessKeyID"
	// AccessKeySecret is a constant for the key in a cloud provider secret and backup secret that holds the Alicloud access key secret.
	AccessKeySecret = "accessKeySecret"
)
