// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Webhook", func() {
	Describe("#New", func() {
		var mgr manager.Manager

		BeforeEach(func() {
			mgr = &test.FakeManager{
				Scheme: kubernetesscheme.Scheme,
			}
		})

		It("should successfully return a webhook object", func() {
			webhook, err := New(mgr, Args{
				Provider: "test-provider",
				Name:     "webhook-test",
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"some": "value"},
				},
				Path: "/webhook",
				Mutators: map[Mutator][]Type{
					&fakeMutator{}: {{Obj: &corev1.Secret{}}},
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(webhook.Provider).To(Equal("test-provider"))
			Expect(webhook.Name).To(Equal("webhook-test"))
			Expect(webhook.NamespaceSelector).To(Equal(&metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			}))
			Expect(webhook.ObjectSelector).To(Equal(&metav1.LabelSelector{
				MatchLabels: map[string]string{"some": "value"},
			}))
			Expect(webhook.Webhook).NotTo(BeNil())
			Expect(webhook.Types).To(ConsistOf(Type{Obj: &corev1.Secret{}}))
		})

		It("should fail because mutators and validators are configured", func() {
			webhook, err := New(mgr, Args{
				Mutators: map[Mutator][]Type{
					&fakeMutator{}: {{Obj: &corev1.Secret{}}},
				},
				Validators: map[Validator][]Type{
					&fakeValidator{}: {{Obj: &corev1.ConfigMap{}}},
				},
			})

			Expect(webhook).To(BeNil())
			Expect(err).To(MatchError("failed to create webhook because a mixture of mutating and validating functions is not permitted"))
		})
	})
})

type fakeMutator struct{}

func (f *fakeMutator) Mutate(_ context.Context, _, _ client.Object) error {
	return nil
}

type fakeValidator struct{}

func (f *fakeValidator) Validate(_ context.Context, _, _ client.Object) error {
	return nil
}
