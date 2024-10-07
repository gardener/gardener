// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("MetricsServer", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{
			Garden: &garden.Garden{},
			Shoot:  &shoot.Shoot{},
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultMetricsServer", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.31.1",
					},
				},
			})
		})

		It("should successfully create a metrics-server interface", func() {
			kubernetesClient.EXPECT().Client()

			metricsServer, err := botanist.DefaultMetricsServer()
			Expect(metricsServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
