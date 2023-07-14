// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gardeneradmissioncontroller

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	admissioncontrollerv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/admissionpluginsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/auditpolicy"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/internaldomainsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/kubeconfigsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/namespacedeletion"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/resourcesize"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/seedrestriction"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

func (a admissioncontroller) validatingwebhookconfiguration(caSecret *corev1.Secret) *admissionregistrationv1.ValidatingWebhookConfiguration {
	var (
		failurePolicyFail     = admissionregistrationv1.Fail
		sideEffectsNone       = admissionregistrationv1.SideEffectClassNone
		matchPolicyEquivalent = admissionregistrationv1.Equivalent

		caBundle = caSecret.Data[secrets.DataKeyCertificateBundle]
	)

	validatingWebhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: DeploymentName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    "validate-namespace-deletion.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          pointer.Int32(10),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{corev1.GroupName},
						APIVersions: []string{"v1"},
						Resources:   []string{"namespaces"},
					},
				}},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleProject,
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      buildClientConfigURL(namespacedeletion.WebhookPath, a.namespace),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "validate-kubeconfig-secrets.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          pointer.Int32(10),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{corev1.GroupName},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets"},
					},
				}},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: v1beta1constants.GardenRole, Operator: metav1.LabelSelectorOpIn, Values: []string{v1beta1constants.GardenRoleProject}},
						{Key: v1beta1constants.LabelApp, Operator: metav1.LabelSelectorOpNotIn, Values: []string{v1beta1constants.LabelGardener}},
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      buildClientConfigURL(kubeconfigsecret.WebhookPath, a.namespace),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "seed-restriction.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          pointer.Int32(10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{corev1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"secrets", "serviceaccounts"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{rbacv1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"clusterrolebindings"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{coordinationv1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"leases"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{certificatesv1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"certificatesigningrequests"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{gardencorev1beta1.GroupName},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"backupentries", "internalsecrets", "shootstates"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Delete},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{gardencorev1beta1.GroupName},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"backupbuckets"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{gardencorev1beta1.GroupName},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"seeds"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{operationsv1alpha1.GroupName},
							APIVersions: []string{"v1alpha1"},
							Resources:   []string{"bastions"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				MatchPolicy:   &matchPolicyEquivalent,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      buildClientConfigURL(seedrestriction.WebhookPath, a.namespace),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "internal-domain-secret.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          pointer.Int32(10),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{corev1.GroupName},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets"},
					},
				}},
				FailurePolicy: &failurePolicyFail,
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.LabelRole: v1beta1constants.GardenRoleInternalDomain,
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      buildClientConfigURL(internaldomainsecret.WebhookPath, a.namespace),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "audit-policies.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          pointer.Int32(10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{gardencorev1beta1.GroupName},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"shoots"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{corev1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleProject,
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      buildClientConfigURL(auditpolicy.WebhookPath, a.namespace),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "admission-plugin-secret.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          pointer.Int32(10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{corev1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"secrets"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleProject,
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      buildClientConfigURL(admissionpluginsecret.WebhookPath, a.namespace),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
		},
	}

	if a.values.ResourceAdmissionConfiguration != nil {
		validatingWebhook.Webhooks = append(validatingWebhook.Webhooks, admissionregistrationv1.ValidatingWebhook{
			Name:                    "validate-resource-size.gardener.cloud",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			TimeoutSeconds:          pointer.Int32(10),
			Rules:                   buildWebhookConfigRulesForResourceSize(a.values.ResourceAdmissionConfiguration),
			FailurePolicy:           &failurePolicyFail,
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: v1beta1constants.GardenRole, Operator: metav1.LabelSelectorOpIn, Values: []string{v1beta1constants.GardenRoleProject}},
					{Key: v1beta1constants.LabelApp, Operator: metav1.LabelSelectorOpNotIn, Values: []string{v1beta1constants.LabelGardener}},
				},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      buildClientConfigURL(resourcesize.WebhookPath, a.namespace),
				CABundle: caBundle,
			},
			SideEffects: &sideEffectsNone,
		})
	}

	return validatingWebhook
}

func buildWebhookConfigRulesForResourceSize(config *admissioncontrollerv1alpha1.ResourceAdmissionConfiguration) []admissionregistrationv1.RuleWithOperations {
	if config == nil || len(config.Limits) == 0 {
		return nil
	}
	rules := make([]admissionregistrationv1.RuleWithOperations, 0, len(config.Limits))

	for _, limit := range config.Limits {
		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   limit.APIGroups,
				APIVersions: limit.APIVersions,
				Resources:   limit.Resources,
			},
		})
	}

	return rules
}

func buildClientConfigURL(webhookPath, namespace string) *string {
	return pointer.String(fmt.Sprintf("https://%s.%s%s", ServiceName, namespace, webhookPath))
}
