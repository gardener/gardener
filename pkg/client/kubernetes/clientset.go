// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
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
	podExecutor   PodExecutor

	// client is the default controller-runtime client which uses SharedIndexInformers to keep its cache in sync
	client client.Client
	// apiReader is a reader that can be used to read directly from the API server instead of reading from
	// the client's cache.
	apiReader client.Reader
	// cache is the client's cache
	cache cache.Cache

	// startOnce guards starting the cache only once
	startOnce sync.Once

	kubernetes kubernetes.Interface

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
func (c *clientSet) ChartApplier() ChartApplier { return c.chartApplier }

// PodExecutor returns a PodExecutor for executing into pods.
func (c *clientSet) PodExecutor() PodExecutor { return c.podExecutor }

// RESTConfig will return the config attribute of the Client object.
func (c *clientSet) RESTConfig() *rest.Config {
	return c.config
}

// Client returns the controller-runtime client of this ClientSet.
func (c *clientSet) Client() client.Client {
	return c.client
}

// APIReader returns a client.Reader that directly reads from the API server.
func (c *clientSet) APIReader() client.Reader {
	return c.apiReader
}

// Cache returns the ClientSet's controller-runtime cache. It can be used to get Informers for arbitrary objects.
func (c *clientSet) Cache() cache.Cache {
	return c.cache
}

// Kubernetes will return the kubernetes attribute of the Client object.
func (c *clientSet) Kubernetes() kubernetes.Interface {
	return c.kubernetes
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

	if err := kubernetesversion.CheckIfSupported(serverVersion.GitVersion); err != nil {
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
				logf.Log.Error(err, "Failed to start the cache, which should never happen, ignoring")
			}
		}()
	})
}

// WaitForCacheSync waits for the cache of the ClientSet's controller-runtime client to be synced.
func (c *clientSet) WaitForCacheSync(ctx context.Context) bool {
	return c.cache.WaitForCacheSync(ctx)
}
