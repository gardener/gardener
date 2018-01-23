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

package awsbotanist

import (
	"github.com/gardener/gardener/pkg/client/aws"
	"github.com/gardener/gardener/pkg/operation"
)

// AWSBotanist is a struct which has methods that perform AWS cloud-specific operations for a Shoot cluster.
type AWSBotanist struct {
	*operation.Operation
	CloudProviderName string
	AWSClient         aws.ClientInterface
}

const (
	// AccessKeyID is a constant for the key in a cloud provider secret that holds the AWS access key id.
	AccessKeyID = "accessKeyID"
	// SecretAccessKey is a constant for the key in a cloud provider secret that holds the AWS secret access key.
	SecretAccessKey = "secretAccessKey"
)
