// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rolloutspeedup

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

// WebhookName is the name of the rollout-speedup webhook. It modifies the API server deployments (kube-apiserver and
// garden-apiserver) to make sure that a rolling update happens faster.
const WebhookName = "rollout-speedup"

var (
	logger = log.Log.WithName("local-rollout-speedup-webhook")

	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the local exposure webhook to the manager.
type AddOptions struct{}

// AddToManagerWithOptions creates a webhook with the given options and adds it to the manager.
func AddToManagerWithOptions(mgr manager.Manager, _ AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	var (
		name     = "rollout-speedup"
		provider = local.Type
		types    = []extensionswebhook.Type{
			{Obj: &appsv1.Deployment{}},
		}
	)

	logger = logger.WithValues("provider", provider)

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(&mutator{client: mgr.GetClient()}, types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook", "name", name)

	return &extensionswebhook.Webhook{
		Name:              name,
		Provider:          provider,
		Types:             types,
		Target:            extensionswebhook.TargetSeed,
		Path:              name,
		Webhook:           &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}},
		ObjectSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: v1beta1constants.LabelRole, Operator: metav1.LabelSelectorOpIn, Values: []string{v1beta1constants.LabelAPIServer}},
		}},
	}, nil
}

// AddToManager creates a webhook with the default options and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
