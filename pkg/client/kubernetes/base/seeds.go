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

package kubernetesbase

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
)

// UpdateSeedStatus update an existing Seed resource's status.
func (c *Client) UpdateSeedStatus(seed *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error) {
	return c.GardenClientset.GardenV1beta1().Seeds().UpdateStatus(seed)
}
