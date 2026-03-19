// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap

import (
	"context"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// NewSelfHostedShootClientMap returns a ClientMap that always returns the given clientSet. This is used for self-hosted
// shoot clusters where the shoot IS the local (seed) cluster — no remote kubeconfig lookup is needed.
func NewSelfHostedShootClientMap(clientSet kubernetes.Interface) ClientMap {
	return &selfHostedShootClientMap{clientSet: clientSet}
}

type selfHostedShootClientMap struct {
	clientSet kubernetes.Interface
}

func (m *selfHostedShootClientMap) GetClient(_ context.Context, _ ClientSetKey) (kubernetes.Interface, error) {
	return m.clientSet, nil
}

func (m *selfHostedShootClientMap) InvalidateClient(_ ClientSetKey) error {
	return nil
}

func (m *selfHostedShootClientMap) Start(_ context.Context) error {
	return nil
}
