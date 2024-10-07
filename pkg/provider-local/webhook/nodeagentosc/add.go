// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentosc

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

const (
	// WebhookName is the name of the webhook for mutating the gardener-node-agent files in OperatingSystemConfigs.
	WebhookName = "operatingsystemconfig-gardener-node-agent"
)

var (
	logger = log.Log.WithName("local-operatingsystemconfig-gardener-node-agent-webhook")

	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the webhook to the manager.
type AddOptions struct{}

// AddToManagerWithOptions creates a webhook with the given options and adds it to the manager.
func AddToManagerWithOptions(
	mgr manager.Manager,
	_ AddOptions,
	name string,
	target string,
	failurePolicy admissionregistrationv1.FailurePolicyType,
) (
	*extensionswebhook.Webhook,
	error,
) {
	logger.Info("Adding webhook to manager")

	var (
		provider = local.Type
		types    = []extensionswebhook.Type{{Obj: &extensionsv1alpha1.OperatingSystemConfig{}}}
	)

	logger = logger.WithValues("provider", provider)

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(&mutator{}, types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook", "name", name)

	return &extensionswebhook.Webhook{
		Name:           name,
		Provider:       provider,
		Types:          types,
		Target:         target,
		Path:           name,
		Webhook:        &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		FailurePolicy:  &failurePolicy,
		TimeoutSeconds: ptr.To[int32](5),
		NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: v1beta1constants.LabelShootProvider, Operator: metav1.LabelSelectorOpIn, Values: []string{provider}},
		}},
	}, nil
}

// AddToManager creates a webhook with the default options and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOptions(
		mgr,
		DefaultAddOptions,
		WebhookName,
		extensionswebhook.TargetSeed,
		admissionregistrationv1.Fail,
	)
}
