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
	"k8s.io/apimachinery/pkg/types"
)

// CreateShoot creates a new Shoot resource.
func (c *Client) CreateShoot(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	newShoot, err := c.
		GardenClientset.
		GardenV1beta1().
		Shoots(shoot.ObjectMeta.Namespace).
		Create(shoot)
	return newShoot, err
}

// GetShoot returns a Shoot resource.
func (c *Client) GetShoot(namespace, name string) (*gardenv1beta1.Shoot, error) {
	return c.
		GardenClientset.
		GardenV1beta1().
		Shoots(namespace).
		Get(name, metav1.GetOptions{})
}

// PatchShoot patches an existing Shoot resource.
func (c *Client) PatchShoot(shoot *gardenv1beta1.Shoot, body []byte) (*gardenv1beta1.Shoot, error) {
	newShoot, err := c.
		GardenClientset.
		GardenV1beta1().
		Shoots(shoot.ObjectMeta.Namespace).
		Patch(shoot.ObjectMeta.Name, types.JSONPatchType, body)
	if err != nil && apierrors.IsNotFound(err) {
		return c.CreateShoot(shoot)
	}
	return newShoot, err
}

// UpdateShoot update an existing Shoot resource.
func (c *Client) UpdateShoot(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	return c.
		GardenClientset.
		GardenV1beta1().
		Shoots(shoot.ObjectMeta.Namespace).
		Update(shoot)
}

// UpdateShootStatus update an existing Shoot resource's status.
func (c *Client) UpdateShootStatus(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	return c.
		GardenClientset.
		GardenV1beta1().
		Shoots(shoot.ObjectMeta.Namespace).
		UpdateStatus(shoot)
}

// ListShoots returns a list of shoot resources in a given <namespace>.
func (c *Client) ListShoots(namespace string) (*gardenv1beta1.ShootList, error) {
	return c.
		GardenClientset.
		GardenV1beta1().
		Shoots(namespace).
		List(metav1.ListOptions{})
}

// DeleteShoot deletes an existing Shoot resource.
func (c *Client) DeleteShoot(namespace, name string) error {
	return c.
		GardenClientset.
		GardenV1beta1().
		Shoots(namespace).
		Delete(name, &defaultDeleteOptions)
}
