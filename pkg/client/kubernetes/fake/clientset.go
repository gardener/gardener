// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fake

import (
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/version"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ kubernetes.Interface = &ClientSet{}

type ClientSet struct {
	applier         kubernetes.Applier
	chartRenderer   chartrenderer.Interface
	chartApplier    kubernetes.ChartApplier
	restConfig      *rest.Config
	client          client.Client
	directClient    client.Client
	cache           cache.Cache
	restMapper      meta.RESTMapper
	kubernetes      kubernetesclientset.Interface
	gardenCore      gardencoreclientset.Interface
	apiextension    apiextensionsclientset.Interface
	apiregistration apiregistrationclientset.Interface
	restClient      rest.Interface
	version         string
}

// NewClientSet returns a new empty fake ClientSet.
func NewClientSet() *ClientSet {
	return &ClientSet{}
}

// Applier returns the applier of this ClientSet.
func (c *ClientSet) Applier() kubernetes.Applier {
	return c.applier
}

// ChartRenderer returns a ChartRenderer populated with the cluster's Capabilities.
func (c *ClientSet) ChartRenderer() chartrenderer.Interface {
	return c.chartRenderer
}

// ChartApplier returns a ChartApplier using the ClientSet's ChartRenderer and Applier.
func (c *ClientSet) ChartApplier() kubernetes.ChartApplier {
	return c.chartApplier
}

// RESTConfig will return the restConfig attribute of the Client object.
func (c *ClientSet) RESTConfig() *rest.Config {
	return c.restConfig
}

// Client returns the controller-runtime client of this ClientSet.
func (c *ClientSet) Client() client.Client {
	return c.client
}

// DirectClient returns a controller-runtime client, which can be used to talk to the API server directly
// (without using a cache).
func (c *ClientSet) DirectClient() client.Client {
	return c.directClient
}

// Cache returns the clientset's controller-runtime cache. It can be used to get Informers for arbitrary objects.
func (c *ClientSet) Cache() cache.Cache {
	return c.cache
}

// RESTMapper returns the restMapper of this ClientSet.
func (c *ClientSet) RESTMapper() meta.RESTMapper {
	return c.restMapper
}

// Kubernetes will return the kubernetes attribute of the Client object.
func (c *ClientSet) Kubernetes() kubernetesclientset.Interface {
	return c.kubernetes
}

// GardenCore will return the gardenCore attribute of the Client object.
func (c *ClientSet) GardenCore() gardencoreclientset.Interface {
	return c.gardenCore
}

// APIExtension will return the apiextension ClientSet attribute of the Client object.
func (c *ClientSet) APIExtension() apiextensionsclientset.Interface {
	return c.apiextension
}

// APIRegistration will return the apiregistration attribute of the Client object.
func (c *ClientSet) APIRegistration() apiregistrationclientset.Interface {
	return c.apiregistration
}

// RESTClient will return the restClient attribute of the Client object.
func (c *ClientSet) RESTClient() rest.Interface {
	return c.restClient
}

// Version returns the GitVersion of the Kubernetes client stored on the object.
func (c *ClientSet) Version() string {
	return c.version
}

// DiscoverVersion tries to retrieve the server version using the kubernetes discovery client.
func (c *ClientSet) DiscoverVersion() (*version.Info, error) {
	serverVersion, err := c.Kubernetes().Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	c.version = serverVersion.GitVersion
	return serverVersion, nil
}

// Start does nothing as the fake ClientSet does not support it.
func (c *ClientSet) Start(<-chan struct{}) {
}

// WaitForCacheSync does nothing and return trues.
func (c *ClientSet) WaitForCacheSync(<-chan struct{}) bool {
	return true
}

// ForwardPodPort does nothing as the fake ClientSet does not support it.
func (c *ClientSet) ForwardPodPort(string, string, int, int) (chan struct{}, error) {
	return nil, nil
}

// CheckForwardPodPort does nothing as the fake ClientSet does not support it.
func (c *ClientSet) CheckForwardPodPort(string, string, int, int) error {
	return nil
}
