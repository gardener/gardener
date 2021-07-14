// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cloudprovider

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// Ensurer ensures that the cloudprovider secret conforms to the provider requirements.
type Ensurer interface {
	EnsureCloudProviderSecret(ctx context.Context, gctx gcontext.GardenContext, new, old *corev1.Secret) error
}

// NewMutator creates a new cloudprovider mutator.
func NewMutator(logger logr.Logger, ensurer Ensurer) webhook.Mutator {
	return &mutator{
		logger:  logger.WithName("mutator"),
		ensurer: ensurer,
	}
}

type mutator struct {
	client  client.Client
	logger  logr.Logger
	ensurer Ensurer
}

// InjectClient injects the client into the ensurer.
func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	if _, err := inject.ClientInto(client, m.ensurer); err != nil {
		return fmt.Errorf("could not inject the client into the ensurer: %w", err)
	}
	return nil
}

// InjectScheme injects the manager's scheme into the ensurer.
func (m *mutator) InjectScheme(scheme *runtime.Scheme) error {
	if _, err := inject.SchemeInto(scheme, m.ensurer); err != nil {
		return fmt.Errorf("could not inject scheme into the ensurer: %w", err)
	}
	return nil
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

	etcx := gcontext.NewGardenContext(m.client, new)
	webhook.LogMutation(m.logger, newSecret.Kind, newSecret.Namespace, newSecret.Name)
	return m.ensurer.EnsureCloudProviderSecret(ctx, etcx, newSecret, oldSecret)
}
