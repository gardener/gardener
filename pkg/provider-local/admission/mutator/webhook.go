// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

const (
	// Name is a name for a validation webhook.
	Name = "mutator"
)

var logger = log.Log.WithName("local-mutator-webhook")

// New creates a new webhook that mutates Shoot resources.
func New(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Setting up webhook", "name", Name)

	return extensionswebhook.New(mgr, extensionswebhook.Args{
		Provider:   local.Type,
		Name:       Name,
		Path:       "/webhooks/mutate",
		Predicates: []predicate.Predicate{extensionspredicate.GardenCoreProviderType(local.Type)},
		Mutators: map[extensionswebhook.Mutator][]extensionswebhook.Type{
			NewShootMutator(mgr): {{Obj: &gardencorev1beta1.Shoot{}}},
		},
		Target: extensionswebhook.TargetSeed,
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"provider.extensions.gardener.cloud/local": "true"},
		},
	})
}
