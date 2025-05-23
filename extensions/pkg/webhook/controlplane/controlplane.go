// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	// WebhookName is the webhook name.
	WebhookName = "controlplane"
	// SeedProviderWebhookName is the seed provider webhook name.
	SeedProviderWebhookName = "seedprovider"
	// BackupWebhookName is the backup webhook name.
	BackupWebhookName = "controlplanebackup"

	// KindSeed - A controlplane seed webhook is applied only to those shoot namespaces that have the correct Seed provider label.
	KindSeed = "seed"
	// KindShoot - A controlplane shoot webhook is applied only to those shoot namespaces that have the correct Shoot provider label.
	KindShoot = "shoot"
	// KindBackup - A controlplane backup webhook is applied only to those shoot namespaces that have the correct Backup provider label.
	KindBackup = "backup"
)

var logger = log.Log.WithName("controlplane-webhook")

// Args are arguments for creating a controlplane webhook.
type Args struct {
	// Kind is the kind of this webhook
	Kind string
	// Provider is the provider of this webhook.
	Provider string
	// Types is a list of resource types.
	Types []extensionswebhook.Type
	// Mutator is a mutator to be used by the admission handler.
	Mutator extensionswebhook.Mutator
	// ObjectSelector is the object selector of the underlying webhook
	ObjectSelector *metav1.LabelSelector
}

// New creates a new controlplane webhook with the given args.
func New(mgr manager.Manager, args Args) (*extensionswebhook.Webhook, error) {
	logger := logger.WithValues("kind", args.Kind, "provider", args.Provider)

	// Create handler
	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(args.Mutator, args.Types...).Build()
	if err != nil {
		return nil, err
	}

	// Create webhook
	logger.Info("Creating webhook", "name", getName(args.Kind))

	// Build namespace selector from the webhook kind and provider
	namespaceSelector, err := buildNamespaceSelector(args.Kind, args.Provider)
	if err != nil {
		return nil, err
	}

	return &extensionswebhook.Webhook{
		Name:              getName(args.Kind),
		Provider:          args.Provider,
		Types:             args.Types,
		Target:            extensionswebhook.TargetSeed,
		Path:              getName(args.Kind),
		Webhook:           &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		NamespaceSelector: namespaceSelector,
		ObjectSelector:    args.ObjectSelector,
	}, nil
}

func getName(kind string) string {
	switch kind {
	case KindSeed:
		return SeedProviderWebhookName
	case KindBackup:
		return BackupWebhookName
	default:
		return WebhookName
	}
}

// buildNamespaceSelector creates and returns a LabelSelector for the given webhook kind and provider.
func buildNamespaceSelector(kind, provider string) (*metav1.LabelSelector, error) {
	// Determine label selector key from the kind
	var key string

	switch kind {
	case KindSeed:
		key = v1beta1constants.LabelSeedProvider
	case KindShoot:
		key = v1beta1constants.LabelShootProvider
	case KindBackup:
		key = v1beta1constants.LabelBackupProvider
	default:
		return nil, fmt.Errorf("invalid webhook kind '%s'", kind)
	}

	// Create and return LabelSelector
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: key, Operator: metav1.LabelSelectorOpIn, Values: []string{provider}},
		},
	}, nil
}
