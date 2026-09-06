// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// WebhookName is the name of the kube-proxy-config webhook.
const WebhookName = "kube-proxy-config"

var logger = log.Log.WithName("local-seed-webhook")

// AddToManager creates a webhook and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	var (
		provider = local.Type
		types    = []extensionswebhook.Type{{Obj: &corev1.Secret{}}}
	)

	logger = logger.WithValues("provider", provider)

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(NewMutator(), types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook", "name", WebhookName)

	return &extensionswebhook.Webhook{
		Name:    WebhookName,
		Types:   types,
		Target:  extensionswebhook.TargetSeed,
		Path:    WebhookName,
		Webhook: &admission.Webhook{Handler: handler, RecoverPanic: new(true)},
		NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: v1beta1constants.LabelSeedProvider, Operator: metav1.LabelSelectorOpIn, Values: []string{provider}},
		}},
		// Select only the kube-proxy ManagedResource secrets. The label is set by
		// getManagedResourceLabels in pkg/component/kubernetes/proxy/proxy.go ("component: kube-proxy").
		// The central kube-proxy secret carries the kube-proxy ConfigMap whose conntrack maxPerCore
		// must be disabled on the local provider.
		ObjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"component": "kube-proxy"}},
	}, nil
}
