// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

// ClientMapBuilder can build a fake ClientMap which can be used to fake ClientMaps in unit tests.
type ClientMapBuilder struct {
	clientSets map[clientmap.ClientSetKey]kubernetes.Interface
}

// NewClientMapBuilder constructs a new ClientMapBuilder.
func NewClientMapBuilder() *ClientMapBuilder {
	return &ClientMapBuilder{
		clientSets: make(map[clientmap.ClientSetKey]kubernetes.Interface),
	}
}

// WithClientSets set the map of ClientSets, that should be contained in the ClientMap.
func (b *ClientMapBuilder) WithClientSets(clientSets map[clientmap.ClientSetKey]kubernetes.Interface) *ClientMapBuilder {
	b.clientSets = clientSets
	return b
}

// WithClientSetForKey adds a given ClientSet for the given key to the map of ClientSets,
// that should be contained in the ClientMap.
func (b *ClientMapBuilder) WithClientSetForKey(key clientmap.ClientSetKey, cs kubernetes.Interface) *ClientMapBuilder {
	b.clientSets[key] = cs
	return b
}

// WithRuntimeClientForKey adds a ClientSet containing only the given controller-runtime Client for the given key
// to the map of ClientSets, that should be contained in the ClientMap.
func (b *ClientMapBuilder) WithRuntimeClientForKey(key clientmap.ClientSetKey, c client.Client, config *rest.Config) *ClientMapBuilder {
	b.clientSets[key] = fake.NewClientSetBuilder().WithClient(c).WithRESTConfig(config).Build()
	return b
}

// Build builds the ClientMap using the provided attributes.
func (b *ClientMapBuilder) Build() *ClientMap {
	return NewClientMapWithClientSets(b.clientSets)
}
