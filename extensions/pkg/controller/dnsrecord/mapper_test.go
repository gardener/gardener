// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Mapper", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client

		namespace = "some-namespace"
		cluster   *extensionsv1alpha1.Cluster

		dnsRecord  *extensionsv1alpha1.DNSRecord
		dnsRecord2 *extensionsv1alpha1.DNSRecord
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		dnsRecord = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dns-record-1",
				Namespace: namespace,
			},
		}

		dnsRecord2 = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dns-record-2",
				Namespace: namespace,
			},
		}
	})

	Describe("#ClusterToDNSRecordMapper", func() {
		var mapper handler.MapFunc

		BeforeEach(func() {
			mapper = ClusterToDNSRecordMapper(fakeClient, nil)
		})

		It("should find all DNSRecord objects for the passed cluster", func() {
			Expect(fakeClient.Create(ctx, dnsRecord)).To(Succeed())
			Expect(fakeClient.Create(ctx, dnsRecord2)).To(Succeed())

			Expect(mapper(ctx, cluster)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      dnsRecord.Name,
						Namespace: namespace,
					},
				},
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      dnsRecord2.Name,
						Namespace: namespace,
					},
				},
			))
		})

		It("should find no objects for the passed cluster because predicates do not match", func() {
			mapper = ClusterToDNSRecordMapper(fakeClient, []predicate.Predicate{
				predicate.Funcs{GenericFunc: func(_ event.GenericEvent) bool {
					return false
				}},
			})

			Expect(fakeClient.Create(ctx, dnsRecord)).To(Succeed())

			Expect(mapper(ctx, cluster)).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			Expect(mapper(ctx, cluster)).To(BeEmpty())
		})

		It("should find no objects because the passed object is not a Cluster", func() {
			Expect(mapper(ctx, dnsRecord)).To(BeEmpty())
		})

		It("should not return DNSRecords from a different namespace", func() {
			otherDNSRecord := &extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dns-record-other",
					Namespace: "other-namespace",
				},
			}
			Expect(fakeClient.Create(ctx, dnsRecord)).To(Succeed())
			Expect(fakeClient.Create(ctx, otherDNSRecord)).To(Succeed())

			result := mapper(ctx, cluster)
			Expect(result).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      dnsRecord.Name,
						Namespace: namespace,
					},
				},
			))
		})
	})
})
