// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/testing"
)

var _ kubernetesclientset.Interface = &ClientSet{}

// ClientSet implements k8s.io/client-go/kubernetes.Interface but allows to use a fake discovery client.
type ClientSet struct {
	kubernetesclientset.Interface
	discovery.DiscoveryInterface
}

// NewClientSetWithDiscovery allows to easily fake calls to kubernetes.Interface.Discovery() by using the given
// discovery interface.
func NewClientSetWithDiscovery(kubernetes kubernetesclientset.Interface, discovery discovery.DiscoveryInterface) *ClientSet {
	return &ClientSet{kubernetes, discovery}
}

// NewClientSetWithFakedServerVersion allows to easily fake calls to kubernetes.Interface.Discovery().ServerVersion()
// by using the given version.
func NewClientSetWithFakedServerVersion(kubernetes kubernetesclientset.Interface, version *version.Info) *ClientSet {
	return &ClientSet{
		Interface: kubernetes,
		DiscoveryInterface: &fakediscovery.FakeDiscovery{
			Fake:               &testing.Fake{},
			FakedServerVersion: version,
		},
	}
}

// Discovery returns the discovery interface of this client set.
func (c *ClientSet) Discovery() discovery.DiscoveryInterface {
	return c.DiscoveryInterface
}
