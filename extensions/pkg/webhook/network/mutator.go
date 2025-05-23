// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// MutateFn is a function that validates and if needed mutates the given extensionsv1alpha1.Network.
type MutateFn func(new, old *extensionsv1alpha1.Network) error

// NewMutator creates a new network mutator.
func NewMutator(mgr manager.Manager, logger logr.Logger, mutateFn MutateFn) webhook.Mutator {
	return &mutator{
		client:     mgr.GetClient(),
		logger:     logger.WithName("mutator"),
		mutateFunc: mutateFn,
	}
}

type mutator struct {
	client     client.Client
	logger     logr.Logger
	mutateFunc MutateFn
}

// Mutate validates and if needed mutates the given object.
func (m *mutator) Mutate(_ context.Context, new, old client.Object) error {
	var (
		newNetwork, oldNetwork *extensionsv1alpha1.Network
		ok                     bool
	)

	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	newNetwork, ok = new.(*extensionsv1alpha1.Network)
	if !ok {
		return fmt.Errorf("could not mutate, object is not of type %q", "Network")
	}

	if old != nil {
		oldNetwork, ok = old.(*extensionsv1alpha1.Network)
		if !ok {
			return errors.New("could not cast old object to extensionsv1alpha1.Network")
		}
	}

	return m.mutateFunc(newNetwork, oldNetwork)
}
