// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	// WebhookName is the name of the shoot webhook.
	WebhookName = "shoot"
	// KindSystem is used for webhooks which should only apply to the to the kube-system namespace.
	KindSystem = "system"
)

var logger = log.Log.WithName("shoot-webhook")

// Args are arguments for creating a webhook targeting a shoot.
type Args struct {
	// Types is a list of resource types.
	Types []extensionswebhook.Type
	// Mutator is a mutator to be used by the admission handler.
	// If a shoot client is needed, the webhook.WantsShootClient interface must be implemented. A client.Client is then
	// injected into the context under the webhook.ShootClientContextKey key.
	Mutator extensionswebhook.Mutator
	// MutatorWithShootClient is a mutator to be used by the admission handler. It needs the shoot client.
	MutatorWithShootClient extensionswebhook.MutatorWithShootClient
	// ObjectSelector is the object selector of the underlying webhook.
	ObjectSelector *metav1.LabelSelector
	// FailurePolicy is the failure policy for the webhook (defaults to Ignore).
	FailurePolicy *admissionregistrationv1.FailurePolicyType
}

// New creates a new webhook with the shoot as target cluster.
func New(mgr manager.Manager, args Args) (*extensionswebhook.Webhook, error) {
	logger.Info("Creating webhook", "name", WebhookName)

	wh := &extensionswebhook.Webhook{
		Name:              WebhookName,
		Types:             args.Types,
		Path:              WebhookName,
		Target:            extensionswebhook.TargetShoot,
		NamespaceSelector: buildNamespaceSelector(),
		ObjectSelector:    args.ObjectSelector,
		FailurePolicy:     args.FailurePolicy,
	}

	var (
		handler admission.Handler
		err     error
	)

	switch {
	case args.Mutator != nil:
		handler, err = extensionswebhook.NewBuilder(mgr, logger).WithMutator(args.Mutator, args.Types...).Build()
	case args.MutatorWithShootClient != nil:
		handler, err = extensionswebhook.NewHandlerWithShootClient(mgr, args.Types, args.MutatorWithShootClient, logger)
	}

	if err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, fmt.Errorf("neither mutator nor mutator with shoot client is set")
	}

	wh.Webhook = &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)}
	return wh, nil
}

// buildNamespaceSelector creates and returns a LabelSelector for the given webhook kind and provider.
func buildNamespaceSelector() *metav1.LabelSelector {
	// Create and return LabelSelector
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: v1beta1constants.GardenerPurpose, Operator: metav1.LabelSelectorOpIn, Values: []string{metav1.NamespaceSystem}},
		},
	}
}
