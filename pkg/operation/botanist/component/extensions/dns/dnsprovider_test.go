// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dns_test

import (
	"context"
	"fmt"
	"time"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dns"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#DNSProvider", func() {
	const (
		deployNS        = "test-chart-namespace"
		secretName      = "extensions-dns-test-deploy"
		dnsProviderName = "test-deploy"
	)

	var (
		ctrl      *gomock.Controller
		ctx       context.Context
		c         client.Client
		scheme    *runtime.Scheme
		fakeOps   *retryfake.Ops
		now       time.Time
		resetVars func()

		expectedSecret   *corev1.Secret
		expected         *dnsv1alpha1.DNSProvider
		vals             *ProviderValues
		log              logrus.FieldLogger
		defaultDepWaiter component.DeployWaiter
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		now = time.Now()
		fakeOps = &retryfake.Ops{MaxAttempts: 1}
		resetVars = test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
			&TimeNow, func() time.Time { return now },
		)

		ctx = context.TODO()
		log = logger.NewNopLogger()

		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(dnsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())

		c = fake.NewClientBuilder().WithScheme(scheme).Build()

		expectedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: deployNS,
			},
			Data: map[string][]byte{
				"some-data": []byte("foo"),
			},
			Type: corev1.SecretTypeOpaque,
		}

		vals = &ProviderValues{
			Name:     "test-deploy",
			Purpose:  "test",
			Provider: "some-emptyProvider",
			SecretData: map[string][]byte{
				"some-data": []byte("foo"),
			},
			Domains: &IncludeExclude{
				Include: []string{"foo.com"},
				Exclude: []string{"baz.com"},
			},
			Zones: &IncludeExclude{
				Include: []string{"goo.local"},
				Exclude: []string{"dodo.local"},
			},
		}

		expected = &dnsv1alpha1.DNSProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dnsProviderName,
				Namespace: deployNS,
				Annotations: map[string]string{
					"gardener.cloud/timestamp": now.UTC().String(),
				},
			},
			Spec: dnsv1alpha1.DNSProviderSpec{
				Type: "some-emptyProvider",
				SecretRef: &corev1.SecretReference{
					Name: secretName,
				},
				Domains: &dnsv1alpha1.DNSSelection{
					Include: []string{"foo.com"},
					Exclude: []string{"baz.com"},
				},
				Zones: &dnsv1alpha1.DNSSelection{
					Include: []string{"goo.local"},
					Exclude: []string{"dodo.local"},
				},
			},
		}

		defaultDepWaiter = NewProvider(log, c, deployNS, vals)
	})

	AfterEach(func() {
		resetVars()
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		DescribeTable("correct DNSProvider is created",
			func(mutator func()) {
				mutator()

				Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())

				actual := &dnsv1alpha1.DNSProvider{}
				err := c.Get(ctx, client.ObjectKey{Name: dnsProviderName, Namespace: deployNS}, actual)

				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(DeepDerivativeEqual(expected))
			},

			Entry("with no modification", func() {}),
			Entry("with no domains", func() {
				vals.Domains = nil
				expected.Spec.Domains = nil
			}),
			Entry("with no exclude domain", func() {
				vals.Domains.Exclude = nil
				expected.Spec.Domains.Exclude = nil
			}),
			Entry("with no zones", func() {
				vals.Zones = nil
				expected.Spec.Zones = nil
			}),
			Entry("with custom labels", func() {
				vals.Labels = map[string]string{"foo": "bar"}
				expected.ObjectMeta.Labels = map[string]string{"foo": "bar"}
			}),
			Entry("with custom annotations", func() {
				vals.Annotations = map[string]string{"foo": "bar"}
				expected.ObjectMeta.Annotations = map[string]string{"foo": "bar"}
			}),
			Entry("with no exclude zones", func() {
				vals.Zones.Exclude = nil
				expected.Spec.Zones.Exclude = nil
			}),
			Entry("with internal prupose", func() {
				vals.Purpose = "internal"
				expected.Annotations = nil
			}),
		)

		DescribeTable("correct Secret is created", func(mutator func()) {
			mutator()

			Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())

			actual := &corev1.Secret{}
			err := c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: deployNS}, actual)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(DeepDerivativeEqual(expectedSecret))
		},
			Entry("with no modification", func() {}),
			Entry("with custom labels", func() {
				vals.Labels = map[string]string{"foo": "bar"}
				expectedSecret.ObjectMeta.Labels = map[string]string{"foo": "bar"}
			}),
		)
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing emptyEntry succeeds")

			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should not return error when it's deleted successfully", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Delete(ctx, &dnsv1alpha1.DNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dnsProviderName,
					Namespace: deployNS,
				}}).Times(1).Return(fmt.Errorf("some random error"))

			Expect(NewProvider(log, mc, deployNS, vals).Destroy(ctx)).To(HaveOccurred())
		})
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should retry getting object if it does not exist in the cache yet", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Scheme().Return(scheme).AnyTimes()

			expected.Status.State = "Ready"
			gomock.InOrder(
				mc.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(expected)).
					Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("dnsproviders"), expected.Name)),
				mc.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(expected)).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *dnsv1alpha1.DNSProvider) error {
						expected.DeepCopyInto(obj)
						return nil
					}),
			)

			fakeOps.MaxAttempts = 2
			defaultDepWaiter = NewProvider(log, mc, deployNS, vals)
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())
		})

		It("should return error when it's not ready", func() {
			expected.Status.State = "dummy-not-ready"
			expected.Status.Message = pointer.String("some-error-message")

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing emptyProvider succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			By("deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			expected.Status.State = "Ready"
			// add old timestamp annotation
			expected.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().String(),
			}
			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching dnsprovider succeeds")

			By("wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "dnsprovider indicates error")
		})

		It("should return no error when it's ready", func() {
			By("deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			expected.Status.State = "Ready"
			// add up-to-date timestamp annotation
			expected.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}
			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching dnsprovider succeeds")

			By("wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "dnsprovider is ready")
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
		})
	})
})
