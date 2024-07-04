// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/webhook/defaulting"
	"github.com/gardener/gardener/pkg/operator/webhook/defaulting/extension"
	"github.com/gardener/gardener/pkg/operator/webhook/defaulting/garden"
	"github.com/gardener/gardener/pkg/operator/webhook/validation"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(mgr manager.Manager) error {
	if err := defaulting.AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding defaulting webhook handlers: %w", err)
	}

	if err := (&validation.Handler{
		Logger:        mgr.GetLogger().WithName("webhook").WithName(validation.HandlerName),
		RuntimeClient: mgr.GetClient(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", validation.HandlerName, err)
	}

	return nil
}

// GetValidatingWebhookConfiguration returns the webhook configuration for the given mode and URL.
func GetValidatingWebhookConfiguration(mode, url string) *admissionregistrationv1.ValidatingWebhookConfiguration {
	var (
		sideEffects   = admissionregistrationv1.SideEffectClassNone
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
	)

	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-operator",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    "validation.operator.gardener.cloud",
				ClientConfig:            getClientConfig(validation.WebhookPath, mode, url),
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{operatorv1alpha1.SchemeGroupVersion.Group},
						APIVersions: []string{operatorv1alpha1.SchemeGroupVersion.Version},
						Resources:   []string{"gardens"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
					},
				}},
				SideEffects:    &sideEffects,
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				TimeoutSeconds: ptr.To[int32](10),
			},
		},
	}
}

// GetMutatingWebhookConfiguration returns the webhook configuration for the given mode and URL.
func GetMutatingWebhookConfiguration(mode, url string) *admissionregistrationv1.MutatingWebhookConfiguration {
	var (
		sideEffects   = admissionregistrationv1.SideEffectClassNone
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
	)

	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-operator",
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				Name:                    "garden.defaulting.operator.gardener.cloud",
				ClientConfig:            getClientConfig(garden.WebhookPath, mode, url),
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{operatorv1alpha1.SchemeGroupVersion.Group},
						APIVersions: []string{operatorv1alpha1.SchemeGroupVersion.Version},
						Resources:   []string{"gardens"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
					},
				}},
				SideEffects:    &sideEffects,
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				TimeoutSeconds: ptr.To[int32](10),
			},
			{
				Name:                    "extension.defaulting.operator.gardener.cloud",
				ClientConfig:            getClientConfig(extension.WebhookPath, mode, url),
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{operatorv1alpha1.SchemeGroupVersion.Group},
						APIVersions: []string{operatorv1alpha1.SchemeGroupVersion.Version},
						Resources:   []string{"extensions"},
					},
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
					},
				}},
				SideEffects:    &sideEffects,
				FailurePolicy:  &failurePolicy,
				MatchPolicy:    &matchPolicy,
				TimeoutSeconds: ptr.To[int32](10),
			},
		},
	}
}

func getClientConfig(webhookPath, mode, url string) admissionregistrationv1.WebhookClientConfig {
	return webhook.BuildClientConfigFor(
		webhookPath,
		v1beta1constants.GardenNamespace,
		"gardener-operator",
		443,
		mode,
		url,
		nil,
	)
}
