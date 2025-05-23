// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockplutono "github.com/gardener/gardener/pkg/component/observability/plutono/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Plutono", func() {
	var (
		ctrl *gomock.Controller

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		mockPlutono *mockplutono.MockInterface

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

		mockPlutono = mockplutono.NewMockInterface(ctrl)

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
							Plutono: mockPlutono,
						},
					},
				},
			},
		}

		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Status: gardencorev1beta1.SeedStatus{
				KubernetesVersion: ptr.To("1.2.3"),
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

	Describe("#DeployPlutono", func() {
		It("should successfully deploy plutono", func() {
			mockPlutono.EXPECT().Deploy(ctx)
			Expect(botanist.DeployPlutono(ctx)).To(Succeed())
		})

		It("should successfully destroy plutono", func() {
			botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeTesting
			mockPlutono.EXPECT().Destroy(ctx)
			Expect(botanist.DeployPlutono(ctx)).To(Succeed())
		})
	})
})
