// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// WebhookName is the name of the Prometheus webhook.
const WebhookName = "prometheus"

var (
	logger = log.Log.WithName("local-prometheus-webhook")

	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the prometheus webhook to the manager.
type AddOptions struct {
	RemoteWriteURLs []string
	ExternalLabels  map[string]string
}

// AddToManagerWithOptions creates a webhook with the given options and adds it to the manager.
func AddToManagerWithOptions(mgr manager.Manager, opts AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	var (
		provider = local.Type
		types    = []extensionswebhook.Type{
			{Obj: &monitoringv1.Prometheus{}},
		}
	)

	logger = logger.WithValues("provider", provider)

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(&mutator{
		client:          mgr.GetClient(),
		remoteWriteURLs: opts.RemoteWriteURLs,
		externalLabels:  opts.ExternalLabels,
	}, types...).Build()
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
		ObjectSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: v1beta1constants.LabelApp, Operator: metav1.LabelSelectorOpIn, Values: []string{"prometheus"}},
			{Key: v1beta1constants.LabelRole, Operator: metav1.LabelSelectorOpIn, Values: []string{v1beta1constants.LabelMonitoring}},
			{Key: "name", Operator: metav1.LabelSelectorOpIn, Values: sets.List(handledPrometheusNames)},
		}},
	}, nil
}

// AddToManager creates a webhook with the default options and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
