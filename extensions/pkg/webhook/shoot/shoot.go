// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"net/http"

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
	// ObjectSelector is the object selector of the underlying webhook.
	ObjectSelector *metav1.LabelSelector
	// FailurePolicy is the failure policy for the webhook (defaults to Ignore).
	FailurePolicy *admissionregistrationv1.FailurePolicyType
}

// New creates a new webhook with the shoot as target cluster.
func New(mgr manager.Manager, args Args) (*extensionswebhook.Webhook, error) {
	logger.Info("Creating webhook", "name", WebhookName)

	if args.Mutator == nil {
		return nil, fmt.Errorf("no mutator is set")
	}

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(args.Mutator, args.Types...).Build()
	if err != nil {
		return nil, err
	}

	return &extensionswebhook.Webhook{
		Name:              WebhookName,
		Types:             args.Types,
		Path:              WebhookName,
		Target:            extensionswebhook.TargetShoot,
		NamespaceSelector: buildNamespaceSelector(),
		ObjectSelector:    args.ObjectSelector,
		FailurePolicy:     args.FailurePolicy,
		Webhook: &admission.Webhook{
			Handler:         handler,
			RecoverPanic:    ptr.To(true),
			WithContextFunc: injectRemoteAddrIntoContextFunc(args.Mutator),
		},
	}, nil
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

func injectRemoteAddrIntoContextFunc(mutator extensionswebhook.Mutator) func(context.Context, *http.Request) context.Context {
	wantsShootClient, ok1 := mutator.(extensionswebhook.WantsShootClient)
	wantsClusterObject, ok2 := mutator.(extensionswebhook.WantsClusterObject)

	if (ok1 && wantsShootClient.WantsShootClient()) ||
		(ok2 && wantsClusterObject.WantsClusterObject()) {
		logger.Info("Setting function for injecting the remote address into context for shoot webhook since it either wants a shoot client or a Cluster object")

		return func(ctx context.Context, request *http.Request) context.Context {
			if request != nil {
				ctx = context.WithValue(ctx, extensionswebhook.RemoteAddrContextKey{}, request.RemoteAddr) //nolint:staticcheck
			}
			return ctx
		}
	}

	return nil
}
