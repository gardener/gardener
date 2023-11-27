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
				Selector: &metav1.LabelSelector{
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
			Expect(webhook.Selector).To(Equal(&metav1.LabelSelector{
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
