// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
