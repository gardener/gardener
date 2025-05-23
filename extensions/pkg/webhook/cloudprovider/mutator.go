// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// Ensurer ensures that the cloudprovider secret conforms to the provider requirements.
type Ensurer interface {
	EnsureCloudProviderSecret(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *corev1.Secret) error
}

// NewMutator creates a new cloudprovider mutator.
func NewMutator(mgr manager.Manager, logger logr.Logger, ensurer Ensurer) webhook.Mutator {
	return &mutator{
		client:  mgr.GetClient(),
		logger:  logger.WithName("mutator"),
		ensurer: ensurer,
	}
}

type mutator struct {
	client  client.Client
	logger  logr.Logger
	ensurer Ensurer
}

// Mutate validates and if needed mutates the given object.
func (m *mutator) Mutate(ctx context.Context, new, old client.Object) error {
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	newSecret, ok := new.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type %q", "Secret")
	}
	if newSecret.Name != v1beta1constants.SecretNameCloudProvider {
		return nil
	}

	var oldSecret *corev1.Secret
	if old != nil {
		oldSecret, ok = old.(*corev1.Secret)
		if !ok {
			return fmt.Errorf("could not mutate: old object could not be casted to type %q", "Secret")
		}
	}

	etcx := extensionscontextwebhook.NewGardenContext(m.client, new)
	webhook.LogMutation(m.logger, newSecret.Kind, newSecret.Namespace, newSecret.Name)
	return m.ensurer.EnsureCloudProviderSecret(ctx, etcx, newSecret, oldSecret)
}
