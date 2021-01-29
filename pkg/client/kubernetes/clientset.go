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

package kubernetes

import (
	"context"
	"sync"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/logger"

	apiextensionclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// clientSet is a struct containing the configuration for the respective Kubernetes
// cluster, the collection of Kubernetes clients <ClientSet> containing all REST clients
// for the built-in Kubernetes API groups, and the Garden which is a REST clientSet
// for the Garden API group.
// The RESTClient itself is a normal HTTP client for the respective Kubernetes cluster,
// allowing requests to arbitrary URLs.
// The version string contains only the major/minor part in the form <major>.<minor>.
type clientSet struct {
	config     *rest.Config
	restClient rest.Interface

	applier       Applier
	chartApplier  ChartApplier
	chartRenderer chartrenderer.Interface

	// client is the default controller-runtime client which uses SharedIndexInformers to keep its cache in sync
	client client.Client
	// directClient is a client which can be used to make requests directly to the API server instead of reading from
	// the client's cache
	directClient client.Client
	// cache is the client's cache
	cache cache.Cache

	// startOnce guards starting the cache only once
	startOnce sync.Once

	kubernetes      kubernetes.Interface
	gardenCore      gardencoreclientset.Interface
	apiextension    apiextensionclientset.Interface
	apiregistration apiregistrationclientset.Interface

	version string
}

// Applier returns the Applier of this ClientSet.
func (c *clientSet) Applier() Applier {
	return c.applier
}

// ChartRenderer returns a ChartRenderer populated with the cluster's Capabilities.
func (c *clientSet) ChartRenderer() chartrenderer.Interface {
	return c.chartRenderer
}

// ChartApplier returns a ChartApplier using the ClientSet's ChartRenderer and Applier.
func (c *clientSet) ChartApplier() ChartApplier {
	return c.chartApplier
}

// RESTConfig will return the config attribute of the Client object.
func (c *clientSet) RESTConfig() *rest.Config {
	return c.config
}

// Client returns the controller-runtime client of this ClientSet.
func (c *clientSet) Client() client.Client {
	return c.client
}

// DirectClient returns a controller-runtime client, which can be used to talk to the API server directly
// (without using a cache).
func (c *clientSet) DirectClient() client.Client {
	return c.directClient
}

// Cache returns the ClientSet's controller-runtime cache. It can be used to get Informers for arbitrary objects.
func (c *clientSet) Cache() cache.Cache {
	return c.cache
}

// Kubernetes will return the kubernetes attribute of the Client object.
func (c *clientSet) Kubernetes() kubernetes.Interface {
	return c.kubernetes
}

// GardenCore will return the gardenCore attribute of the Client object.
func (c *clientSet) GardenCore() gardencoreclientset.Interface {
	return c.gardenCore
}

// APIExtension will return the apiextensions attribute of the Client object.
func (c *clientSet) APIExtension() apiextensionclientset.Interface {
	return c.apiextension
}

// APIRegistration will return the apiregistration attribute of the Client object.
func (c *clientSet) APIRegistration() apiregistrationclientset.Interface {
	return c.apiregistration
}

// RESTClient will return the restClient attribute of the Client object.
func (c *clientSet) RESTClient() rest.Interface {
	return c.restClient
}

// Version returns the GitVersion of the Kubernetes client stored on the object.
func (c *clientSet) Version() string {
	return c.version
}

// DiscoverVersion tries to retrieve the server version of the targeted Kubernetes cluster and updates the
// ClientSet's saved version accordingly. Use Version if you only want to retrieve the kubernetes version instead
// of refreshing the ClientSet's saved version.
func (c *clientSet) DiscoverVersion() (*version.Info, error) {
	serverVersion, err := c.kubernetes.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	if err := checkIfSupportedKubernetesVersion(serverVersion.GitVersion); err != nil {
		return nil, err
	}

	c.version = serverVersion.GitVersion
	c.chartRenderer = chartrenderer.NewWithServerVersion(serverVersion)
	c.chartApplier = NewChartApplier(c.chartRenderer, c.applier)

	return serverVersion, nil
}

// Start starts the cache of the ClientSet's controller-runtime client and returns immediately.
// It must be called first before using the client to retrieve objects from the API server.
func (c *clientSet) Start(ctx context.Context) {
	c.startOnce.Do(func() {
		go func() {
			if err := c.cache.Start(ctx); err != nil {
				logger.Logger.Errorf("cache.Start returned error, which should never happen, ignoring.")
			}
		}()
	})
}

// WaitForCacheSync waits for the cache of the ClientSet's controller-runtime client to be synced.
func (c *clientSet) WaitForCacheSync(ctx context.Context) bool {
	return c.cache.WaitForCacheSync(ctx)
}
