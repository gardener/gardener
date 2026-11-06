// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("StaticPods", func() {
	var (
		seedClient client.Client
		botanist   *Botanist
	)

	BeforeEach(func() {
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{
			Operation: &operation.Operation{
				SeedClientSet: fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build(),
				Shoot:         &shootpkg.Shoot{ControlPlaneNamespace: "kube-system"},
			},
		}
	})

	Describe("#useShootAccessTokensForSelfHostedShootControlPlane", func() {
		selfHostedShoot := func(gardenerName string) *gardencorev1beta1.Shoot {
			return &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
						}},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Gardener: gardencorev1beta1.Gardener{Name: gardenerName},
				},
			}
		}

		It("should return false for a non-self-hosted shoot", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when the shoot status indicates gardenadm", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenadm"))

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when gardener-resource-manager deployment does not exist", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when gardener-resource-manager runs with host network (bootstrap phase)", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: "kube-system"},
				Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: true}}},
			}
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return true when gardener-resource-manager runs in the pod network", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: "kube-system"},
				Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: false}}},
			}
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeTrue())
		})
	})
})
