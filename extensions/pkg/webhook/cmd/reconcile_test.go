// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("reconcileSeedWebhookConfig", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		mgr        *test.FakeManager
		config     *AddToManagerConfig
		caBundle   = []byte("ca-bundle")
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		mgr = &test.FakeManager{Client: fakeClient}
		config = &AddToManagerConfig{}
	})

	It("should reconcile both mutating and validating webhook configs", func() {
		mutatingConfig := &admissionregistrationv1.MutatingWebhookConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admissionregistration.k8s.io/v1",
				Kind:       "MutatingWebhookConfiguration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-extension-admission-test",
			},
			Webhooks: []admissionregistrationv1.MutatingWebhook{{}},
		}

		validatingConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admissionregistration.k8s.io/v1",
				Kind:       "ValidatingWebhookConfiguration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-extension-admission-test",
			},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{}},
		}

		webhookConfigs := extensionswebhook.Configs{
			MutatingWebhookConfig:   mutatingConfig,
			ValidatingWebhookConfig: validatingConfig,
		}

		reconcileFn := config.reconcileSeedWebhookConfig(mgr, webhookConfigs, caBundle)
		Expect(reconcileFn(ctx)).To(Succeed())

		createdMutating := &admissionregistrationv1.MutatingWebhookConfiguration{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mutatingConfig), createdMutating)).To(Succeed())
		Expect(createdMutating.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))

		createdValidating := &admissionregistrationv1.ValidatingWebhookConfiguration{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(validatingConfig), createdValidating)).To(Succeed())
		Expect(createdValidating.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
	})
})
