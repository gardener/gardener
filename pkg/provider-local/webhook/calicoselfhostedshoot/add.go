// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package calicoselfhostedshoot

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

// WebhookName is the name of the calico self-hosted shoot webhook.
const WebhookName = "calico-self-hosted-shoot"

var logger = log.Log.WithName("local-calico-self-hosted-shoot-webhook")

// AddToManager creates the webhook and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	var (
		name     = WebhookName
		provider = local.Type
		types    = []extensionswebhook.Type{
			{Obj: &appsv1.DaemonSet{}},
		}
	)

	logger = logger.WithValues("provider", provider)

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(&mutator{client: mgr.GetClient()}, types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook", "name", name)

	return &extensionswebhook.Webhook{
		Name:    name,
		Types:   types,
		Target:  extensionswebhook.TargetShoot,
		Path:    name,
		Webhook: &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
			corev1.LabelMetadataName:    metav1.NamespaceSystem,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
		}},
		ObjectSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "k8s-app", Operator: metav1.LabelSelectorOpIn, Values: []string{"calico-node"}},
		}},
	}, nil
}
