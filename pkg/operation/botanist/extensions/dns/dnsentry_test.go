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
	"reflect"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/extensions/dns"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#DNSEntry", func() {
	const (
		deployNS     = "test-chart-namespace"
		secretName   = "extensions-dns-test-deploy"
		dnsEntryName = "test-deploy"
	)
	var (
		ctrl             *gomock.Controller
		ca               kubernetes.ChartApplier
		ctx              context.Context
		c                client.Client
		expected         *dnsv1alpha1.DNSEntry
		vals             *EntryValues
		log              logrus.FieldLogger
		defaultDepWaiter component.DeployWaiter
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		ctx = context.TODO()
		log = logger.NewNopLogger()

		s := runtime.NewScheme()
		Expect(dnsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewFakeClientWithScheme(s)

		vals = &EntryValues{
			Name:    "test-deploy",
			DNSName: "test-name",
			Targets: []string{"1.2.3.4", "5.6.7.8"},
			TTL:     123,
		}

		expected = &dnsv1alpha1.DNSEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dnsEntryName,
				Namespace: deployNS,
			},
			Spec: dnsv1alpha1.DNSEntrySpec{
				DNSName: "test-name",
				TTL:     &vals.TTL,
				Targets: []string{"1.2.3.4", "5.6.7.8"},
			},
		}

		ca = kubernetes.NewChartApplier(cr.NewWithServerVersion(&version.Info{}), kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(ca).NotTo(BeNil(), "should return chart applier")

		defaultDepWaiter = NewDNSEntry(vals, deployNS, ca, chartsRoot(), log, c, &fakeOps{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		DescribeTable("correct DNSProvider is created", func(mutator func()) {
			mutator()

			Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())

			actual := &dnsv1alpha1.DNSEntry{}
			err := c.Get(ctx, client.ObjectKey{Name: dnsEntryName, Namespace: deployNS}, actual)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(DeepDerivativeEqual(expected))
		},
			Entry("with no modification", func() {}),
			Entry("with ownerID", func() {
				vals.OwnerID = "dummy-owner"
				expected.Spec.OwnerId = pointer.StringPtr("dummy-owner")
			}),
		)
	})

	Describe("#Wait", func() {

		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when it's not ready", func() {
			expected.Status.State = "dummy-not-ready"
			expected.Status.Message = pointer.StringPtr("some-error-message")

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing entry succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return no error when is ready", func() {
			expected.Status.State = "Ready"
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing entry succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).ToNot(HaveOccurred())
		})

		It("should set a default waiter", func() {
			wrt := NewDNSEntry(vals, deployNS, ca, chartsRoot(), log, c, nil)
			Expect(reflect.ValueOf(wrt).Elem().FieldByName("waiter").IsNil()).ToNot(BeTrue())
		})

	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing entry succeeds")

			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should return error when it's not deleted successfully", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Delete(ctx, &dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dnsEntryName,
					Namespace: deployNS,
				}}).Times(1).Return(fmt.Errorf("some random error"))

			defaultDepWaiter = NewDNSEntry(vals, deployNS, ca, chartsRoot(), log, mc, &fakeOps{})

			Expect(defaultDepWaiter.Destroy(ctx)).To(HaveOccurred())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
		})
	})

})
