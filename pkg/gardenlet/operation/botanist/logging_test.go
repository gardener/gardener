// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	mockvali "github.com/gardener/gardener/pkg/component/observability/logging/vali/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Logging", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		k8sSeedClient         kubernetes.Interface
		botanist              *Botanist
		eventLoggerDeployer   *mockcomponent.MockDeployer
		valiDeployer          *mockvali.MockInterface
		fakeSecretManager     secretsmanager.Interface
		chartApplier          *mock.MockChartApplier
		ctx                   = context.TODO()
		controlPlaneNamespace = "shoot--foo--bar"
		shootName             = "bar"
		projectNamespace      = "garden-foo"
		fakeErr               = fmt.Errorf("fake error")

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
		fakeSecretManager = fakesecretsmanager.New(c, controlPlaneNamespace)

		botanist = &Botanist{
			Operation: &operation.Operation{
				Logger:         logr.Discard(),
				SecretsManager: fakeSecretManager,
				SeedClientSet:  k8sSeedClient,
				Config: &gardenletconfigv1alpha1.GardenletConfiguration{
					Logging: &gardenletconfigv1alpha1.Logging{
						Enabled: ptr.To(true),
						Vali: &gardenletconfigv1alpha1.Vali{
							Enabled: ptr.To(true),
						},
						ShootNodeLogging: &gardenletconfigv1alpha1.ShootNodeLogging{
							ShootPurposes: []gardencorev1beta1.ShootPurpose{
								"development",
							},
						},
						ShootEventLogging: &gardenletconfigv1alpha1.ShootEventLogging{
							Enabled: ptr.To(true),
						},
					},
				},
				Seed: &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
					Purpose:               "development",
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
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
				TechnicalID: controlPlaneNamespace,
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
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		It("should successfully delete the logging stack when it is disabled", func() {
			*botanist.Config.Logging.Enabled = false
			gomock.InOrder(
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeployLogging(ctx)).To(Succeed())
		})

		Context("When there are no control plane components yet", func() {
			It("should successfully deploy the logging stack when it is enabled and there is no gardener-resource-manager", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
						_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
						deployment := &appsv1.Deployment{}
						*obj.(*appsv1.Deployment) = *deployment
						return nil
					}),
					valiDeployer.EXPECT().WithAuthenticationProxy(false),

					valiDeployer.EXPECT().Deploy(ctx),
				)

				Expect(botanist.DeployLogging(ctx)).To(Succeed())
			})
			It("should not deploy the logging stack when there is an error fetching gardener-resource-manager", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
						_ runtimeclient.ObjectKey, _ runtimeclient.Object, _ ...runtimeclient.GetOption) error {
						return fakeErr
					}),
				)

				Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
			})
		})

		Context("When gardener-resource-manager is present in the control plane", func() {
			It("should successfully deploy all of the components in the logging stack when it is enabled", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
						_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
						deployment := &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
								Namespace: controlPlaneNamespace,
							},
							Status: appsv1.DeploymentStatus{
								ReadyReplicas: 1,
							},
						}
						*obj.(*appsv1.Deployment) = *deployment
						return nil
					}),
					valiDeployer.EXPECT().WithAuthenticationProxy(true),

					eventLoggerDeployer.EXPECT().Deploy(ctx),
					valiDeployer.EXPECT().Deploy(ctx),
				)

				Expect(botanist.DeployLogging(ctx)).To(Succeed())
			})

			It("should not deploy event logger when it is disabled", func() {
				*botanist.Config.Logging.ShootEventLogging.Enabled = false
				gomock.InOrder(
					c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
						_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
						deployment := &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
								Namespace: controlPlaneNamespace,
							},
							Status: appsv1.DeploymentStatus{
								ReadyReplicas: 1,
							},
						}
						*obj.(*appsv1.Deployment) = *deployment
						return nil
					}),
					valiDeployer.EXPECT().WithAuthenticationProxy(true),

					eventLoggerDeployer.EXPECT().Destroy(ctx),
					valiDeployer.EXPECT().Deploy(ctx),
				)

				Expect(botanist.DeployLogging(ctx)).To(Succeed())
			})

			It("should not deploy shoot node logging for workerless shoot", func() {
				botanist.Shoot.IsWorkerless = true
				gomock.InOrder(
					c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
						_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
						deployment := &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
								Namespace: controlPlaneNamespace,
							},
							Status: appsv1.DeploymentStatus{
								ReadyReplicas: 1,
							},
						}
						*obj.(*appsv1.Deployment) = *deployment
						return nil
					}),
					valiDeployer.EXPECT().WithAuthenticationProxy(true),

					eventLoggerDeployer.EXPECT().Deploy(ctx),
					valiDeployer.EXPECT().Deploy(ctx),
				)

				Expect(botanist.DeployLogging(ctx)).To(Succeed())
			})

			It("should not deploy shoot node logging when it is disabled", func() {
				botanist.Config.Logging.ShootNodeLogging = nil
				gomock.InOrder(
					c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
						_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
						deployment := &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
								Namespace: controlPlaneNamespace,
							},
							Status: appsv1.DeploymentStatus{
								ReadyReplicas: 1,
							},
						}
						*obj.(*appsv1.Deployment) = *deployment
						return nil
					}),
					valiDeployer.EXPECT().WithAuthenticationProxy(true),

					eventLoggerDeployer.EXPECT().Deploy(ctx),
					valiDeployer.EXPECT().Deploy(ctx),
				)

				Expect(botanist.DeployLogging(ctx)).To(Succeed())
			})

			It("should not deploy shoot node logging and Vali when Vali is disabled", func() {
				*botanist.Config.Logging.Vali.Enabled = false
				gomock.InOrder(
					c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
						_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
						deployment := &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
								Namespace: controlPlaneNamespace,
							},
							Status: appsv1.DeploymentStatus{
								ReadyReplicas: 1,
							},
						}
						*obj.(*appsv1.Deployment) = *deployment
						return nil
					}),
					valiDeployer.EXPECT().WithAuthenticationProxy(true),

					eventLoggerDeployer.EXPECT().Deploy(ctx),
					valiDeployer.EXPECT().Destroy(ctx),
				)

				Expect(botanist.DeployLogging(ctx)).To(Succeed())
			})

			Context("Tests expecting a failure", func() {
				It("should fail to delete the logging stack when ShootEventLoggerDeployer Destroy return error", func() {
					*botanist.Config.Logging.Enabled = false
					eventLoggerDeployer.EXPECT().Destroy(ctx).Return(fakeErr)
					Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
				})

				It("should fail to delete the logging stack when logging is disabled and ShootValiDeployer Destroy return error", func() {
					*botanist.Config.Logging.Enabled = false
					gomock.InOrder(
						eventLoggerDeployer.EXPECT().Destroy(ctx),
						valiDeployer.EXPECT().Destroy(ctx).Return(fakeErr),
					)

					Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
				})

				It("should fail to deploy the logging stack when ShootEventLoggerDeployer Deploy returns an error", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
							_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
							deployment := &appsv1.Deployment{
								ObjectMeta: metav1.ObjectMeta{
									Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
									Namespace: controlPlaneNamespace,
								},
								Status: appsv1.DeploymentStatus{
									ReadyReplicas: 1,
								},
							}
							*obj.(*appsv1.Deployment) = *deployment
							return nil
						}),
						valiDeployer.EXPECT().WithAuthenticationProxy(true),

						eventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
					)

					Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
				})

				It("should fail to deploy the logging stack when deploying of the shoot event logging fails", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context,
							_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
							deployment := &appsv1.Deployment{
								ObjectMeta: metav1.ObjectMeta{
									Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
									Namespace: controlPlaneNamespace,
								},
								Status: appsv1.DeploymentStatus{
									ReadyReplicas: 1,
								},
							}
							*obj.(*appsv1.Deployment) = *deployment
							return nil
						}),
						valiDeployer.EXPECT().WithAuthenticationProxy(true),

						eventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
					)

					Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
				})

				It("should fail to deploy the logging stack when ValiDeployer Deploy returns error", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.
							Context,
							_ runtimeclient.ObjectKey, obj runtimeclient.Object, _ ...runtimeclient.GetOption) error {
							deployment := &appsv1.Deployment{
								ObjectMeta: metav1.ObjectMeta{
									Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
									Namespace: controlPlaneNamespace,
								},
								Status: appsv1.DeploymentStatus{
									ReadyReplicas: 1,
								},
							}
							*obj.(*appsv1.Deployment) = *deployment
							return nil
						}),
						valiDeployer.EXPECT().WithAuthenticationProxy(true),

						eventLoggerDeployer.EXPECT().Deploy(ctx),
						valiDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
					)

					Expect(botanist.DeployLogging(ctx)).ToNot(Succeed())
				})
			})
		})
	})
})
