// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"

	"k8s.io/apimachinery/pkg/version"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ kubernetes.Interface = &ClientSet{}

// ClientSet contains information to provide a fake implementation for tests.
type ClientSet struct {
	applier       kubernetes.Applier
	chartRenderer chartrenderer.Interface
	chartApplier  kubernetes.ChartApplier
	podExecutor   kubernetes.PodExecutor
	restConfig    *rest.Config
	client        client.Client
	apiReader     client.Reader
	cache         cache.Cache
	kubernetes    kubernetesclientset.Interface
	restClient    rest.Interface
	version       string
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

// PodExecutor returns a PodExecutor.
func (c *ClientSet) PodExecutor() kubernetes.PodExecutor {
	return c.podExecutor
}

// RESTConfig will return the restConfig attribute of the Client object.
func (c *ClientSet) RESTConfig() *rest.Config {
	return c.restConfig
}

// Client returns the controller-runtime client of this ClientSet.
func (c *ClientSet) Client() client.Client {
	return c.client
}

// APIReader returns a client.Reader that directly reads from the API server.
func (c *ClientSet) APIReader() client.Reader {
	return c.apiReader
}

// Cache returns the clientset's controller-runtime cache. It can be used to get Informers for arbitrary objects.
func (c *ClientSet) Cache() cache.Cache {
	return c.cache
}

// Kubernetes will return the kubernetes attribute of the Client object.
func (c *ClientSet) Kubernetes() kubernetesclientset.Interface {
	return c.kubernetes
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
func (c *ClientSet) Start(context.Context) {
}

// WaitForCacheSync does nothing and return true.
func (c *ClientSet) WaitForCacheSync(context.Context) bool {
	return true
}

// PortForwarder fakes the PortForwarder interface.
type PortForwarder struct {
	Err                 error
	ReadyChan, DoneChan chan struct{}
}

// ForwardPorts returns Err as soon as DoneChan is closed.
func (f PortForwarder) ForwardPorts() error {
	<-f.DoneChan
	return f.Err
}

// Ready returns ReadyChan.
func (f PortForwarder) Ready() chan struct{} {
	return f.ReadyChan
}
