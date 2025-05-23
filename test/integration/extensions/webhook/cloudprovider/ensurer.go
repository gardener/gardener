// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		logger: logger,
	}
}

type ensurer struct {
	logger logr.Logger
}

// EnsureCloudProviderSecret is implemented on extension side which mutates the cloudprovider secret. contain
// For testing purpose we are mutating the cloudprovider secret's data to check whether this
// function is called in webhook.
func (e *ensurer) EnsureCloudProviderSecret(_ context.Context, _ extensionscontextwebhook.GardenContext, new, _ *corev1.Secret) error {
	e.logger.Info("Mutate cloudprovider secret", "namespace", new.Namespace, "name", new.Name)
	new.Data["clientID"] = []byte(`foo`)
	new.Data["clientSecret"] = []byte(`bar`)

	return nil
}
