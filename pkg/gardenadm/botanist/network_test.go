// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		ctx       context.Context
		namespace = "kube-system"

		b *AutonomousBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()

		b = &AutonomousBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Shoot: &shoot.Shoot{
						ControlPlaneNamespace: namespace,
					},
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()).
						Build(),
				},
			},
		}
	})

	Describe("#IsPodNetworkAvailable", func() {
		var (
			managedResource *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-coredns",
					Namespace: namespace,
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
})
