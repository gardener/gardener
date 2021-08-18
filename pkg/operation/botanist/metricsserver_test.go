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

package botanist_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/metricsserver"
	mockmetricsserver "github.com/gardener/gardener/pkg/operation/botanist/component/metricsserver/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("MetricsServer", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultMetricsServer", func() {
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)

			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				DisableDNS: true,
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a metrics-server interface", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.APIServerSNI, true)()

			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: "metrics-server"}}

			metricsServer, err := botanist.DefaultMetricsServer()
			Expect(metricsServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			metricsServer, err := botanist.DefaultMetricsServer()
			Expect(metricsServer).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployMetricsServer", func() {
		var (
			metricsServer *mockmetricsserver.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")

			secretCAName         = "ca-metrics-server"
			secretCAChecksum     = "1234"
			secretServerName     = "metrics-server"
			secretServerChecksum = "5678"
		)

		BeforeEach(func() {
			metricsServer = mockmetricsserver.NewMockInterface(ctrl)

			botanist.StoreCheckSum(secretCAName, secretCAChecksum)
			botanist.StoreCheckSum(secretServerName, secretServerChecksum)
			botanist.StoreSecret(secretCAName, &corev1.Secret{})
			botanist.StoreSecret(secretServerName, &corev1.Secret{})
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					SystemComponents: &shootpkg.SystemComponents{
						MetricsServer: metricsServer,
					},
				},
			}
		})

		BeforeEach(func() {
			metricsServer.EXPECT().SetSecrets(metricsserver.Secrets{
				CA:     component.Secret{Name: secretCAName, Checksum: secretCAChecksum},
				Server: component.Secret{Name: secretServerName, Checksum: secretServerChecksum},
			})
		})

		It("should set the secrets and deploy", func() {
			metricsServer.EXPECT().Deploy(ctx)
			Expect(botanist.DeployMetricsServer(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			metricsServer.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployMetricsServer(ctx)).To(Equal(fakeErr))
		})
	})
})
