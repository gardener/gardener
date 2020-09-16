// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// FactoryAggregator aggregates various Factory functions.
type FactoryAggregator []func(manager.Manager) (*extensionswebhook.Webhook, error)

// NewFactoryAggregator creates a new FactoryAggregator and registers the given functions.
func NewFactoryAggregator(m []func(manager.Manager) (*extensionswebhook.Webhook, error)) FactoryAggregator {
	builder := FactoryAggregator{}

	for _, f := range m {
		builder.Register(f)
	}

	return builder
}

// Register registers the given functions in this builder.
func (a *FactoryAggregator) Register(f func(manager.Manager) (*extensionswebhook.Webhook, error)) {
	*a = append(*a, f)
}

// Webhooks calls all factories with the given managers and returns all created webhooks.
// As soon as there is an error creating a webhook, the error is returned immediately.
func (a *FactoryAggregator) Webhooks(mgr manager.Manager) ([]*extensionswebhook.Webhook, error) {
	webhooks := make([]*extensionswebhook.Webhook, 0, len(*a))

	for _, f := range *a {
		wh, err := f(mgr)
		if err != nil {
			return nil, err
		}

		webhooks = append(webhooks, wh)
	}

	return webhooks, nil
}
