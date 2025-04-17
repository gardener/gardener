// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Network", func() {
	var (
		ctx           context.Context
		namespaceName = "kube-system"

		b *AutonomousBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()

		b = &AutonomousBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Shoot: &shoot.Shoot{
						ControlPlaneNamespace: namespaceName,
						Networks: &shoot.Networks{
							Pods:     []net.IPNet{{IP: net.ParseIP("10.1.2.3"), Mask: net.CIDRMask(8, 32)}},
							Services: []net.IPNet{{IP: net.ParseIP("10.4.5.6"), Mask: net.CIDRMask(8, 32)}},
							Nodes:    []net.IPNet{{IP: net.ParseIP("10.7.8.9"), Mask: net.CIDRMask(8, 32)}},
						},
					},
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()).
						Build(),
				},
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				},
			},
		})
	})

	Describe("#IsPodNetworkAvailable", func() {
		var managedResource *resourcesv1alpha1.ManagedResource

		BeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-coredns",
					Namespace: namespaceName,
				},
			}
		})

		It("should return false because the ManagedResource does not exist", func() {
			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeFalse())
		})

		It("should return false because the ManagedResource is unhealthy", func() {
			Expect(b.SeedClientSet.Client().Create(ctx, managedResource)).To(Succeed())

			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeFalse())
		})

		It("should return true because the ManagedResource is healthy", func() {
			managedResource.Status.ObservedGeneration = managedResource.Generation
			managedResource.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               "ResourcesHealthy",
					Status:             "True",
					LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
					LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
				},
				{
					Type:               "ResourcesApplied",
					Status:             "True",
					LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
					LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
				},
			}
			Expect(b.SeedClientSet.Client().Create(ctx, managedResource)).To(Succeed())

			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeTrue())
		})
	})

	Describe("#ApplyNetworkPolicies", func() {
		It("should apply the NetworkPolicies", func() {
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", Labels: map[string]string{"gardener.cloud/role": "shoot"}}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}
			Expect(b.SeedClientSet.Client().Create(ctx, namespace)).To(Succeed())

			endpoints := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"}}
			Expect(b.SeedClientSet.Client().Create(ctx, endpoints)).To(Succeed())

			networkPolicyList := &networkingv1.NetworkPolicyList{}
			Expect(b.SeedClientSet.Client().List(ctx, networkPolicyList)).To(Succeed())
			Expect(networkPolicyList.Items).To(BeEmpty())

			Expect(b.ApplyNetworkPolicies(ctx)).To(Succeed())

			Expect(b.SeedClientSet.Client().List(ctx, networkPolicyList)).To(Succeed())
			Expect(networkPolicyList.Items).NotTo(BeEmpty())
		})
	})
})
