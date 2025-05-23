// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	terminalv1alpha1 "github.com/gardener/terminal-controller-manager/api/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils"
)

var webhookLabels = map[string]string{
	"app.kubernetes.io/name":      "terminal",
	"app.kubernetes.io/component": "admission-controller",
}

func (t *terminal) mutatingWebhookConfiguration(caBundle []byte) *admissionregistrationv1.MutatingWebhookConfiguration {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "terminal-mutating-webhook-configuration",
			Labels: utils.MergeStringMaps(getLabels(), webhookLabels),
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name:                    "mutating-create-update-terminal.gardener.cloud",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      ptr.To("https://" + name + "." + t.namespace + ".svc/mutate-terminal"),
				CABundle: caBundle,
			},
			FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			SideEffects:   ptr.To(admissionregistrationv1.SideEffectClassNone),
			Rules:         webhookRules,
		}},
	}
}

func (t *terminal) validatingWebhookConfiguration(caBundle []byte) *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "terminal-validating-webhook-configuration",
			Labels: utils.MergeStringMaps(getLabels(), webhookLabels),
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:                    "validating-create-update-terminal.gardener.cloud",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      ptr.To("https://" + name + "." + t.namespace + ".svc/validate-terminal"),
				CABundle: caBundle,
			},
			FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			SideEffects:   ptr.To(admissionregistrationv1.SideEffectClassNone),
			Rules:         webhookRules,
		}},
	}
}

var webhookRules = []admissionregistrationv1.RuleWithOperations{{
	Rule: admissionregistrationv1.Rule{
		APIGroups:   []string{terminalv1alpha1.GroupVersion.Group},
		APIVersions: []string{terminalv1alpha1.GroupVersion.Version},
		Resources:   []string{"terminals"},
	},
	Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
}}
