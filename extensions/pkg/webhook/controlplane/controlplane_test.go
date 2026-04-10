// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("ControlPlane", func() {
	Describe("#New", func() {
		var (
			provider = "provider-test"
			mutator  = &fakeMutator{}
			types    = []extensionswebhook.Type{{Obj: &corev1.Service{}}}
		)

		DescribeTable("should create a webhook with the correct name and namespace selector",
			func(kind, expectedName, expectedLabelKey string) {
				mgr := &test.FakeManager{
					Scheme: kubernetesscheme.Scheme,
				}

				webhook, err := New(mgr, Args{
					Kind:     kind,
					Provider: provider,
					Types:    types,
					Mutator:  mutator,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(webhook).NotTo(BeNil())
				Expect(webhook.Name).To(Equal(expectedName))
				Expect(webhook.Target).To(Equal(extensionswebhook.TargetSeed))
				Expect(webhook.Path).To(Equal(expectedName))
				Expect(webhook.Types).To(Equal(types))
				Expect(webhook.Webhook).NotTo(BeNil())
				Expect(webhook.NamespaceSelector).To(Equal(&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: expectedLabelKey, Operator: metav1.LabelSelectorOpIn, Values: []string{provider}},
					},
				}))
			},
			Entry("shoot kind", KindShoot, WebhookName, v1beta1constants.LabelShootProvider),
			Entry("seed kind", KindSeed, SeedProviderWebhookName, v1beta1constants.LabelSeedProvider),
			Entry("backup kind", KindBackup, BackupWebhookName, v1beta1constants.LabelBackupProvider),
		)

		It("should propagate the ObjectSelector to the webhook", func() {
			mgr := &test.FakeManager{
				Scheme: kubernetesscheme.Scheme,
			}

			objectSelector := &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			}

			webhook, err := New(mgr, Args{
				Kind:           KindShoot,
				Provider:       provider,
				Types:          types,
				Mutator:        mutator,
				ObjectSelector: objectSelector,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(webhook.ObjectSelector).To(Equal(objectSelector))
		})

		It("should fail for an invalid kind", func() {
			mgr := &test.FakeManager{
				Scheme: kubernetesscheme.Scheme,
			}

			webhook, err := New(mgr, Args{
				Kind:     "invalid",
				Provider: provider,
				Types:    types,
				Mutator:  mutator,
			})

			Expect(err).To(MatchError(ContainSubstring("invalid webhook kind")))
			Expect(webhook).To(BeNil())
		})
	})
})

type fakeMutator struct{}

func (f *fakeMutator) Mutate(_ context.Context, _, _ client.Object) error {
	return nil
}
