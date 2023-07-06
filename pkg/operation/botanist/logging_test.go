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

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockvali "github.com/gardener/gardener/pkg/component/logging/vali/mock"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Logging", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		k8sSeedClient          kubernetes.Interface
		botanist               *Botanist
		shootRBACProxyDeployer *mockcomponent.MockDeployer
		eventLoggerDeployer    *mockcomponent.MockDeployer
		valiDeployer           *mockvali.MockInterface
		fakeSecretManager      secretsmanager.Interface
		chartApplier           *mock.MockChartApplier
		ctx                    = context.TODO()
		seedNamespace          = "shoot--foo--bar"
		shootName              = "bar"
		projectNamespace       = "garden-foo"
		fakeErr                = fmt.Errorf("fake error")

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

		shootRBACProxyDeployer = mockcomponent.NewMockDeployer(ctrl)
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
						Enabled: pointer.Bool(true),
						Vali: &config.Vali{
							Enabled: pointer.Bool(true),
						},
						ShootNodeLogging: &config.ShootNodeLogging{
							ShootPurposes: []gardencore.ShootPurpose{
								"development",
							},
						},
						ShootEventLogging: &config.ShootEventLogging{
							Enabled: pointer.Bool(true),
						},
					},
				},
				Seed: &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
					Purpose:       "development",
					Components: &shootpkg.Components{
						Logging: &shootpkg.Logging{
							ShootRBACProxy: shootRBACProxyDeployer,
							EventLogger:    eventLoggerDeployer,
							Vali:           valiDeployer,
						},
					},
					IsWorkerless: false,
				},
				ImageVector: imagevector.ImageVector{
					{Name: "alpine"},
					{Name: "vali"},
					{Name: "vali-curator"},
					{Name: "kube-rbac-proxy"},
					{Name: "telegraf"},
					{Name: "tune2fs"},
				},
			},
		}

		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Status: gardencorev1beta1.SeedStatus{
				KubernetesVersion: pointer.String("1.2.3"),
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

	Describe("#DeploySeedLogging", func() {
		It("should successfully delete the logging stack when shoot is with testing purpose", func() {
			botanist.Shoot.Purpose = shootPurposeTesting
			gomock.InOrder(
				// Destroying the Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				// Destroying the Shoot Event Logging
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				// Delete Vali
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should successfully delete the logging stack when it is disabled", func() {
			*botanist.Config.Logging.Enabled = false
			gomock.InOrder(
				// Destroying the Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				// Destroying the Shoot Event Logging
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				// Delete Vali
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should successfully clean up the existing Loki based deployment and deploy all of the components in the logging stack when it is enabled", func() {
			deleteOptions := []interface{}{
				client.InNamespace(seedNamespace),
				client.MatchingLabels{
					v1beta1constants.GardenRole: "logging",
					v1beta1constants.LabelApp:   "loki",
				}}
			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(nil),

				// Destroying the Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: seedNamespace}}),
				// Delete Loki
				c.EXPECT().Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "loki-0", Namespace: seedNamespace}}, client.GracePeriodSeconds(5)),

				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-promtail", Namespace: seedNamespace}}),
				c.EXPECT().DeleteAllOf(ctx, &corev1.ConfigMap{}, deleteOptions...),

				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Do(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					pvc := obj.(*corev1.PersistentVolumeClaim)
					pvc.Spec.VolumeName = "volumeIDofLoki"
					return nil
				}),

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: seedNamespace, Name: "loki-0"}, gomock.AssignableToTypeOf(&corev1.Pod{})).Return(nil),
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: seedNamespace, Name: "loki-0"}, gomock.AssignableToTypeOf(&corev1.Pod{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Pod"}, "loki-0")),

				c.EXPECT().Get(ctx, client.ObjectKey{Name: "volumeIDofLoki"}, gomock.AssignableToTypeOf(&corev1.PersistentVolume{})).Do(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					pv := obj.(*corev1.PersistentVolume)
					pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
					return nil
				}),

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolume{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					pv := obj.(*corev1.PersistentVolume)
					Expect(pv.Spec.PersistentVolumeReclaimPolicy).To(Equal(corev1.PersistentVolumeReclaimRetain))
					return nil
				}),

				c.EXPECT().Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: seedNamespace}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "volumeIDofLoki"}}),

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(nil),
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),

				c.EXPECT().Get(ctx, client.ObjectKey{Name: "volumeIDofLoki"}, gomock.AssignableToTypeOf(&corev1.PersistentVolume{})).Return(nil),

				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolume{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					pv := obj.(*corev1.PersistentVolume)
					Expect(pv.Spec.ClaimRef).To(BeNil())
					return nil
				}),

				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Do(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					pvc := obj.(*corev1.PersistentVolumeClaim)
					Expect(pvc.ObjectMeta.Name).To(Equal("vali-vali-0"))
					return nil
				}),

				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: seedNamespace, Name: "vali-vali-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "vali-vali-0")),
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: seedNamespace, Name: "vali-vali-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(nil),
				c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: seedNamespace, Name: "vali-vali-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Do(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					pvc := obj.(*corev1.PersistentVolumeClaim)
					pvc.Status.Phase = corev1.ClaimBound
					return nil
				}),

				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PersistentVolume{}), gomock.Any()).Do(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					pv := obj.(*corev1.PersistentVolume)
					Expect(pv.Spec.PersistentVolumeReclaimPolicy).To(Equal(corev1.PersistentVolumeReclaimDelete))
					return nil
				}),

				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				shootRBACProxyDeployer.EXPECT().Deploy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should successfully deploy all of the components in the logging stack when it is enabled", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				shootRBACProxyDeployer.EXPECT().Deploy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should not deploy event logger when it is disabled", func() {
			*botanist.Config.Logging.ShootEventLogging.Enabled = false
			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
				// destroy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Destroy(ctx),
				// deploy Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Deploy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging for workerless shoot", func() {
			botanist.Shoot.IsWorkerless = true
			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				// destroy Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging when it is disabled", func() {
			botanist.Config.Logging.ShootNodeLogging = nil
			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				// destroy Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Deploy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging and Vali when Vali is disabled", func() {
			*botanist.Config.Logging.Vali.Enabled = false
			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
				// deploy Shoot Event Logging
				eventLoggerDeployer.EXPECT().Deploy(ctx),
				// destroy Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				// deploy Vali
				valiDeployer.EXPECT().Destroy(ctx),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		Context("Tests expecting a failure", func() {
			It("should fail to delete the logging stack when ShootRBACProxyDeployer Destroy returns error", func() {
				*botanist.Config.Logging.Enabled = false
				shootRBACProxyDeployer.EXPECT().Destroy(ctx).Return(fakeErr)
				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to delete the logging stack when ShootEventLoggerDeployer Destroy return error", func() {
				*botanist.Config.Logging.Enabled = false
				gomock.InOrder(
					// Destroying the Shoot Node Logging
					shootRBACProxyDeployer.EXPECT().Destroy(ctx),
					// Destroying the Shoot Event Logging
					eventLoggerDeployer.EXPECT().Destroy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to delete the logging stack when logging is disbaled and ShootValiDeployer Destroy return error", func() {
				*botanist.Config.Logging.Enabled = false
				gomock.InOrder(
					// Destroying the Shoot Node Logging
					shootRBACProxyDeployer.EXPECT().Destroy(ctx),
					// Destroying the Shoot Event Logging
					eventLoggerDeployer.EXPECT().Destroy(ctx),
					// Delete Vali
					valiDeployer.EXPECT().Destroy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when ShootEventLoggerDeployer Deploy returns an error", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
					eventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when deploying of the shoot event logging fails", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
					// deploy Shoot Event Logging
					eventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when KubeRBACProxyDeployer Deploy returns an error", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
					eventLoggerDeployer.EXPECT().Deploy(ctx),
					shootRBACProxyDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when ValiDeployer Deploy returns error", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: "loki-loki-0"}, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "PersistentVolumeClaim"}, "loki-loki-0")),
					eventLoggerDeployer.EXPECT().Deploy(ctx),
					shootRBACProxyDeployer.EXPECT().Deploy(ctx),
					valiDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})
		})
	})
})
