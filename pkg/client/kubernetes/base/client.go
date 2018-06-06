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

package kubernetesbase

import (
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	machineclientset "github.com/gardener/gardener/pkg/client/machine/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// DiscoverAPIGroups will fetch all Kubernetes server resources, i.e. all registered API groups and the
// associated resources.
func (c *Client) DiscoverAPIGroups() error {
	apiResourceList, err := c.clientset.Discovery().ServerResources()
	c.apiResourceList = apiResourceList
	return err
}

// GetAPIResourceList will return the Kubernetes API resource list.
func (c *Client) GetAPIResourceList() []*metav1.APIResourceList {
	return c.apiResourceList
}

// GetConfig will return the config attribute of the Client object.
func (c *Client) GetConfig() *rest.Config {
	return c.config
}

// GetResourceAPIGroups will return the resourceAPIGroups attribute of the Client object.
func (c *Client) GetResourceAPIGroups() map[string][]string {
	return c.resourceAPIGroups
}

// Clientset will return the clientset attribute of the Client object.
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// GardenClientset will return the gardenClientset attribute of the Client object.
func (c *Client) GardenClientset() *gardenclientset.Clientset {
	return c.gardenClientset
}

// MachineClientset will return the machineClientset attribute of the Client object.
func (c *Client) MachineClientset() *machineclientset.Clientset {
	return c.machineClientset
}

// RESTClient will return the restClient attribute of the Client object.
func (c *Client) RESTClient() rest.Interface {
	return c.restClient
}

// SetConfig will set the config attribute of the Client object.
func (c *Client) SetConfig(config *rest.Config) {
	c.config = config
}

// SetClientset will set the clientset attribute of the Client object.
func (c *Client) SetClientset(clientset *kubernetes.Clientset) {
	c.clientset = clientset
}

// SetGardenClientset will set the gardenClientset attribute of the Client object.
func (c *Client) SetGardenClientset(client *gardenclientset.Clientset) {
	c.gardenClientset = client
}

// SetMachineClientset will set the machineClientset attribute of the Client object.
func (c *Client) SetMachineClientset(client *machineclientset.Clientset) {
	c.machineClientset = client
}

// SetRESTClient will set the restClient attribute of the Client object.
func (c *Client) SetRESTClient(client rest.Interface) {
	c.restClient = client
}

// SetResourceAPIGroups set the resourceAPIGroups attribute of the Client object.
func (c *Client) SetResourceAPIGroups(groups map[string][]string) {
	c.resourceAPIGroups = groups
}
