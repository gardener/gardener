// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	mockvali "github.com/gardener/gardener/pkg/component/observability/logging/vali/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Logging", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		k8sSeedClient       kubernetes.Interface
		botanist            *Botanist
		eventLoggerDeployer *mockcomponent.MockDeployer
		valiDeployer        *mockvali.MockInterface
		fakeSecretManager   secretsmanager.Interface
		chartApplier        *mock.MockChartApplier
		ctx                 = context.TODO()
		seedNamespace       = "shoot--foo--bar"
		shootName           = "bar"
		projectNamespace    = "garden-foo"
		fakeErr             = fmt.Errorf("fake error")

		shootPurposeDevelopment = gardencorev1beta1.ShootPurposeDevelopment
		shootPurposeTesting     = gardencorev1beta1.ShootPurposeTesting
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		chartApplier = mock.NewMockChartApplier(ctrl)
		k8sSeedClient = fake.NewClientSetBuilder().
			WithClient(c).
			WithChartApplier(chartApplier).
			WithRESTConfig(&rest.Config{}).
			Build()

		eventLoggerDeployer = mockcomponent.NewMockDeployer(ctrl)
		valiDeployer = mockvali.NewMockInterface(ctrl)
		fakeSecretManager = fakesecretsmanager.New(c, seedNamespace)

		botanist = &Botanist{
			Operation: &operation.Operation{
				Logger:         logr.Discard(),
				SecretsManager: fakeSecretManager,
				SeedClientSet:  k8sSeedClient,
				Config: &config.GardenletConfiguration{
					Logging: &config.Logging{
						Enabled: ptr.To(true),
						Vali: &config.Vali{
							Enabled: ptr.To(true),
						},
						ShootNodeLogging: &config.ShootNodeLogging{
							ShootPurposes: []gardencore.ShootPurpose{
								"development",
							},
						},
						ShootEventLogging: &config.ShootEventLogging{
							Enabled: ptr.To(true),
						},
					},
				},
				Seed: &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
					Purpose:       "development",
					Components: &shootpkg.Components{
						Logging: &shootpkg.Logging{
							EventLogger: eventLoggerDeployer,
							Vali:        valiDeployer,
						},
					},
					IsWorkerless: false,
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
				Purpose: &shootPurposeDevelopment,
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: seedNamespace,
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployLogging", func() {
		It("should successfully delete the logging stack when shoot is with testing purpose", func() {
			botanist.Shoot.Purpose = shootPurposeTesting
			gomock.InOrder(
				// Destroying the Shoot Event Logging
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				// Delete Vali
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		It("should successfully delete the logging stack when it is disabled", func() {
			*botanist.Config.Logging.Enabled = false
			gomock.InOrder(
				// Destroying the Shoot Event Logging
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				// Delete Vali
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		It("should successfully deploy all of the components in the logging stack when it is enabled", func() {
			gomock.InOrder(
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		It("should not deploy event logger when it is disabled", func() {
			*botanist.Config.Logging.ShootEventLogging.Enabled = false
			gomock.InOrder(
				// destroy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging for workerless shoot", func() {
			botanist.Shoot.IsWorkerless = true
			gomock.InOrder(
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging when it is disabled", func() {
			botanist.Config.Logging.ShootNodeLogging = nil
			gomock.InOrder(
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging and Vali when Vali is disabled", func() {
			*botanist.Config.Logging.Vali.Enabled = false
			gomock.InOrder(
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		Context("Tests expecting a failure", func() {
			It("should fail to delete the logging stack when ShootEventLoggerDeployer Destroy return error", func() {
				*botanist.Config.Logging.Enabled = false
				// Destroying the Shoot Event Logging
				eventLoggerDeployer.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to delete the logging stack when logging is disabled and ShootValiDeployer Destroy return error", func() {
				*botanist.Config.Logging.Enabled = false
				gomock.InOrder(
					// Destroying the Shoot Event Logging
					eventLoggerDeployer.EXPECT().Destroy(ctx),
					// Delete Vali
					valiDeployer.EXPECT().Destroy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when ShootEventLoggerDeployer Deploy returns an error", func() {
				gomock.InOrder(
					eventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when deploying of the shoot event logging fails", func() {
				gomock.InOrder(
					// deploy Shoot Event Logging
					eventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when ValiDeployer Deploy returns error", func() {
				gomock.InOrder(
					eventLoggerDeployer.EXPECT().Deploy(ctx),
					valiDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
			})
		})
	})
})
