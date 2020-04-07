// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package network

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/webhook"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MutateFn func(new, old *extensionsv1alpha1.Network) error

// NewMutator creates a new network mutator.
func NewMutator(logger logr.Logger, mutateFn MutateFn) webhook.Mutator {
	return &mutator{
		logger:     logger.WithName("mutator"),
		mutateFunc: mutateFn,
	}
}

type mutator struct {
	client     client.Client
	logger     logr.Logger
	mutateFunc MutateFn
}

// InjectClient injects the given client into the ensurer.
func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	return nil
}

// Mutate validates and if needed mutates the given object.
func (m *mutator) Mutate(ctx context.Context, new, old runtime.Object) error {
	var (
		newNetwork, oldNetwork *extensionsv1alpha1.Network
		ok                     bool
	)

	acc, err := meta.Accessor(new)
	if err != nil {
		return errors.Wrapf(err, "could not create accessor during webhook")
	}
	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if acc.GetDeletionTimestamp() != nil {
		return nil
	}

	newNetwork, ok = new.(*extensionsv1alpha1.Network)
	if !ok {
		return fmt.Errorf("could not mutate, object is not of type \"Network\"")
	}

	if old != nil {
		oldNetwork, ok = old.(*extensionsv1alpha1.Network)
		if !ok {
			return fmt.Errorf("could not cast old object to extensionsv1alpha1.Network")
		}
	}

	return m.mutateFunc(newNetwork, oldNetwork)
}
