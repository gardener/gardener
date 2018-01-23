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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateSeed creates a new Seed resource.
func (c *Client) CreateSeed(seed *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error) {
	newSeed, err := c.GardenClientset.GardenV1beta1().Seeds().Create(seed)
	if apierrors.IsAlreadyExists(err) {
		return c.UpdateSeed(seed)
	}
	return newSeed, err
}

// GetSeed returns a Seed resource.
func (c *Client) GetSeed(name string) (*gardenv1beta1.Seed, error) {
	return c.GardenClientset.GardenV1beta1().Seeds().Get(name, metav1.GetOptions{})
}

// UpdateSeed update an existing Seed resource.
func (c *Client) UpdateSeed(seed *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error) {
	return c.GardenClientset.GardenV1beta1().Seeds().Update(seed)
}

// UpdateSeedStatus update an existing Seed resource's status.
func (c *Client) UpdateSeedStatus(seed *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error) {
	return c.GardenClientset.GardenV1beta1().Seeds().UpdateStatus(seed)
}

// DeleteSeed deletes an existing Seed resource.
func (c *Client) DeleteSeed(name string) error {
	return c.GardenClientset.GardenV1beta1().Seeds().Delete(name, &defaultDeleteOptions)
}
