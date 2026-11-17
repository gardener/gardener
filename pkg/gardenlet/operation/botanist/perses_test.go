// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Perses", func() {
	var (
		ctrl *gomock.Controller

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		mockPerses *mockcomponent.MockDeployWaiter

		botanist *Botanist

		ctx                   = context.TODO()
		projectNamespace      = "garden-foo"
		controlPlaneNamespace = "shoot--foo--bar"
		shootName             = "bar"

		shootPurposeEvaluation = gardencorev1beta1.ShootPurposeEvaluation
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		seedClientSet = fake.NewClientSetBuilder().
			WithClient(seedClient).
			WithRESTConfig(&rest.Config{}).
			Build()

		mockPerses = mockcomponent.NewMockDeployWaiter(ctrl)

		botanist = &Botanist{
			Operation: &operation.Operation{
				GardenClient:  gardenClient,
				SeedClientSet: seedClientSet,
				Config:        &gardenletconfigv1alpha1.GardenletConfiguration{},
				Garden: &garden.Garden{
					Project: &gardencorev1beta1.Project{},
				},
				Seed:                &seedpkg.Seed{},
				SeedNamespaceObject: &corev1.Namespace{},
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
					Networks: &shootpkg.Networks{
						Pods:     []net.IPNet{},
						Services: []net.IPNet{},
					},
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							Perses: mockPerses,
						},
					},
				},
			},
		}

		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Status: gardencorev1beta1.SeedStatus{
				KubernetesVersion: new("1.2.3"),
			},
		})

		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: projectNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Purpose: &shootPurposeEvaluation,
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: controlPlaneNamespace,
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	DescribeTable("#DeployPerses",
		func(featureGateEnabled bool, purpose gardencorev1beta1.ShootPurpose, expectDeploy bool) {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.PersesShoot, featureGateEnabled))
			botanist.Shoot.Purpose = purpose

			if expectDeploy {
				mockPerses.EXPECT().Deploy(ctx)
			} else {
				mockPerses.EXPECT().Destroy(ctx)
			}

			Expect(botanist.DeployPerses(ctx)).To(Succeed())
		},

		Entry("should destroy Perses when the feature gate is disabled", false, gardencorev1beta1.ShootPurposeEvaluation, false),
		Entry("should deploy Perses when the feature gate is enabled", true, gardencorev1beta1.ShootPurposeEvaluation, true),
		Entry("should destroy Perses when the shoot does not need observability components", true, gardencorev1beta1.ShootPurposeTesting, false),
	)

	DescribeTable("#WaitForPerses",
		func(featureGateEnabled bool, purpose gardencorev1beta1.ShootPurpose, expectWait bool) {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.PersesShoot, featureGateEnabled))
			botanist.Shoot.Purpose = purpose

			if expectWait {
				mockPerses.EXPECT().Wait(ctx)
			} else {
				mockPerses.EXPECT().WaitCleanup(ctx)
			}

			Expect(botanist.WaitForPerses(ctx)).To(Succeed())
		},

		Entry("should wait for the cleanup when the feature gate is disabled", false, gardencorev1beta1.ShootPurposeEvaluation, false),
		Entry("should wait for Perses when the feature gate is enabled", true, gardencorev1beta1.ShootPurposeEvaluation, true),
		Entry("should wait for the cleanup when the shoot does not need observability components", true, gardencorev1beta1.ShootPurposeTesting, false),
	)

	Describe("#ComputePersesHost", func() {
		It("should use the full name as prefix", func() {
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Ingress: &gardencorev1beta1.Ingress{Domain: "ingress.seed.example.com"},
				},
			})

			Expect(botanist.ComputePersesHost()).To(Equal("perses-foo--bar.ingress.seed.example.com"))
		})
	})
})
