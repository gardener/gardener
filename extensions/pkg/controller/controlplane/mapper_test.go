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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Mapper", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client

		namespace = "some-namespace"
		cluster   *extensionsv1alpha1.Cluster
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
	})

	Describe("#ClusterToControlPlaneMapper", func() {
		var mapper handler.MapFunc

		BeforeEach(func() {
			mapper = ClusterToControlPlaneMapper(fakeClient, nil)
		})

		It("should find all ControlPlanes for the passed cluster", func() {
			cp1 := &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cp1",
					Namespace: namespace,
				},
			}
			cp2 := &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cp2",
					Namespace: namespace,
				},
			}

			Expect(fakeClient.Create(ctx, cp1)).To(Succeed())
			Expect(fakeClient.Create(ctx, cp2)).To(Succeed())

			Expect(mapper(ctx, cluster)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cp1.Name,
						Namespace: namespace,
					},
				},
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cp2.Name,
						Namespace: namespace,
					},
				},
			))
		})

		It("should find no ControlPlanes because the list is empty", func() {
			Expect(mapper(ctx, cluster)).To(BeEmpty())
		})

		It("should return nil when the passed object is not a Cluster", func() {
			Expect(mapper(ctx, &corev1.Secret{})).To(BeEmpty())
		})

		It("should not return ControlPlanes in a different namespace", func() {
			cpOtherNamespace := &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cp-other",
					Namespace: "other-namespace",
				},
			}
			Expect(fakeClient.Create(ctx, cpOtherNamespace)).To(Succeed())

			Expect(mapper(ctx, cluster)).To(BeEmpty())
		})

		It("should only return ControlPlanes that match the given predicates", func() {
			cp := &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cp-filtered",
					Namespace: namespace,
				},
			}
			Expect(fakeClient.Create(ctx, cp)).To(Succeed())

			// Use a predicate that rejects all objects.
			rejectAllMapper := ClusterToControlPlaneMapper(fakeClient, []predicate.Predicate{
				predicate.Funcs{GenericFunc: func(_ event.GenericEvent) bool {
					return false
				}},
			})

			Expect(rejectAllMapper(ctx, cluster)).To(BeEmpty())
		})

		It("should return ControlPlanes when predicate accepts them", func() {
			cp := &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cp-accepted",
					Namespace: namespace,
				},
			}
			Expect(fakeClient.Create(ctx, cp)).To(Succeed())

			// Use a predicate that accepts all objects.
			acceptAllMapper := ClusterToControlPlaneMapper(fakeClient, []predicate.Predicate{
				predicate.Funcs{GenericFunc: func(_ event.GenericEvent) bool {
					return true
				}},
			})

			Expect(acceptAllMapper(ctx, cluster)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cp.Name,
						Namespace: namespace,
					},
				},
			))
		})
	})
})
