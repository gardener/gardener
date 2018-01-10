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
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Bootstrap will fetch all Kubernetes server resources, i.e. all registered API groups and the
// associated resources.
func (c *Client) Bootstrap() error {
	apiResourceList, err := c.
		Clientset.
		Discovery().
		ServerResources()
	c.apiResourceList = apiResourceList
	return err
}

// GetAPIResourceList will return the Kubernetes API resource list.
func (c *Client) GetAPIResourceList() []*metav1.APIResourceList {
	return c.apiResourceList
}

// GetConfig will return the Config attribute of the Client object.
func (c *Client) GetConfig() *rest.Config {
	return c.Config
}

// GetClientset will return the Clientset attribute of the Client object.
func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.Clientset
}

// GetGardenClientset will return the GardenClientset attribute of the Client object.
func (c *Client) GetGardenClientset() *gardenclientset.Clientset {
	return c.GardenClientset
}

// GetRESTClient will return the RESTClient attribute of the Client object.
func (c *Client) GetRESTClient() rest.Interface {
	return c.RESTClient
}

// SetConfig will set the Config attribute of the Client object.
func (c *Client) SetConfig(config *rest.Config) {
	c.Config = config
}

// SetClientset will set the Clientset attribute of the Client object.
func (c *Client) SetClientset(clientset *kubernetes.Clientset) {
	c.Clientset = clientset
}

// SetGardenClientset will set the GardenClientset attribute of the Client object.
func (c *Client) SetGardenClientset(client *gardenclientset.Clientset) {
	c.GardenClientset = client
}

// SetRESTClient will set the RESTClient attribute of the Client object.
func (c *Client) SetRESTClient(client rest.Interface) {
	c.RESTClient = client
}
