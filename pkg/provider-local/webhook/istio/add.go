// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// WebhookName is the name of the istio-must-run-on-control-plane webhook. It ensures that the istio-ingressgateway
// pods run on the control plane node. This is a temporary workaround to make sure that the virtual garden is reachable
// from outside (if they only run on worker nodes, the pods cannot be reached from outside via 172.18.255.3:443).
// TODO(rfranzke): Remove this webhook once the istio-ingressgateway pods can run on worker nodes.
const WebhookName = "istio-must-run-on-control-plane"

var (
	logger = log.Log.WithName("local-istio-must-run-on-control-plane-webhook")

	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the webhook to the manager.
type AddOptions struct{}

// AddToManagerWithOptions creates a webhook with the given options and adds it to the manager.
func AddToManagerWithOptions(mgr manager.Manager, _ AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	var (
		provider = local.Type
		types    = []extensionswebhook.Type{
			{Obj: &appsv1.Deployment{}},
		}
	)

	logger = logger.WithValues("provider", provider)

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(&mutator{}, types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook", "name", WebhookName)

	return &extensionswebhook.Webhook{
		Name:     WebhookName,
		Provider: provider,
		Types:    types,
		Target:   extensionswebhook.TargetSeed,
		Path:     WebhookName,
		Webhook:  &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: corev1.LabelMetadataName, Operator: metav1.LabelSelectorOpIn, Values: []string{"virtual-garden-istio-ingress", "istio-ingress"}},
		}},
		ObjectSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: v1beta1constants.LabelApp, Operator: metav1.LabelSelectorOpIn, Values: []string{"istio-ingressgateway"}},
		}},
	}, nil
}

// AddToManager creates a webhook with the default options and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
