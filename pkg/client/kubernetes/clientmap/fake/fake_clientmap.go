// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ clientmap.ClientMap = &ClientMap{}

// ClientMap is a simple implementation of clientmap.ClientMap which can be used to fake ClientMaps in unit tests.
type ClientMap struct {
	clientSets map[clientmap.ClientSetKey]kubernetes.Interface
}

// NewClientMap creates a new empty ClientMap.
func NewClientMap() *ClientMap {
	return &ClientMap{
		clientSets: make(map[clientmap.ClientSetKey]kubernetes.Interface),
	}
}

// NewClientMapWithClientSets creates a new ClientMap containing the given ClientSets.
func NewClientMapWithClientSets(clientSets map[clientmap.ClientSetKey]kubernetes.Interface) *ClientMap {
	return &ClientMap{
		clientSets: clientSets,
	}
}

// AddClient adds the given ClientSet to the fake ClientMap with the given key.
func (f *ClientMap) AddClient(key clientmap.ClientSetKey, cs kubernetes.Interface) *ClientMap {
	f.clientSets[key] = cs
	return f
}

// AddRuntimeClient adds a new fake ClientSets containing only the given runtime client to the fake ClientMap with the
// given key.
func (f *ClientMap) AddRuntimeClient(key clientmap.ClientSetKey, c client.Client) *ClientMap {
	f.clientSets[key] = fake.NewClientSetBuilder().WithClient(c).Build()
	return f
}

// GetClient returns the ClientSet for the given key if present.
func (f *ClientMap) GetClient(_ context.Context, key clientmap.ClientSetKey) (kubernetes.Interface, error) {
	if cs, ok := f.clientSets[key]; ok {
		return cs, nil
	}

	return nil, fmt.Errorf("clientSet for key %q not found", key.Key())
}

// InvalidateClient removes the ClientSet for the given key from the ClientMap if present.
func (f *ClientMap) InvalidateClient(key clientmap.ClientSetKey) error {
	delete(f.clientSets, key)

	return nil
}

// Start does nothing, as fake ClientMap does not support it.
func (f *ClientMap) Start(_ context.Context) error {
	return nil
}
