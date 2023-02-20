// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"path/filepath"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/mock"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Logging", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		k8sSeedClient            kubernetes.Interface
		botanist                 *Botanist
		shootRBACProxyDeployer   *mockcomponent.MockDeployer
		shootEventLoggerDeployer *mockcomponent.MockDeployer
		fakeSecretManager        secretsmanager.Interface
		chartApplier             *mock.MockChartApplier
		ctx                      = context.TODO()
		seedNamespace            = "shoot--foo--bar"
		shootName                = "bar"
		projectNamespace         = "garden-foo"
		fakeErr                  = fmt.Errorf("fake error")
		gr                       = schema.GroupResource{Resource: "Secrets"}

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
		shootEventLoggerDeployer = mockcomponent.NewMockDeployer(ctrl)
		fakeSecretManager = fakesecretsmanager.New(c, seedNamespace)

		botanist = &Botanist{
			Operation: &operation.Operation{
				SecretsManager: fakeSecretManager,
				SeedClientSet:  k8sSeedClient,
				Config: &config.GardenletConfiguration{
					Logging: &config.Logging{
						Enabled: pointer.Bool(true),
						Loki: &config.Loki{
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
							ShootRBACProxy:   shootRBACProxyDeployer,
							ShootEventLogger: shootEventLoggerDeployer,
						},
					},
				},
				ImageVector: imagevector.ImageVector{
					{Name: "loki"},
					{Name: "loki-curator"},
					{Name: "kube-rbac-proxy"},
					{Name: "telegraf"},
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
				c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: seedNamespace}}),
				// Destroying the Shoot Event Logging
				shootEventLoggerDeployer.EXPECT().Destroy(ctx),
				// Delete Loki
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: seedNamespace}}),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should successfully delete the logging stack when it is disabled", func() {
			*botanist.Config.Logging.Enabled = false
			gomock.InOrder(
				// Destroying the Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: seedNamespace}}),
				// Destroying the Shoot Event Logging
				shootEventLoggerDeployer.EXPECT().Destroy(ctx),
				// Delete Loki
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: seedNamespace}}),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should successfully deploy all of the components in the logging stack when it is enabled", func() {
			gomock.InOrder(
				// deploy Shoot Event Logging
				shootEventLoggerDeployer.EXPECT().Deploy(ctx),
				shootRBACProxyDeployer.EXPECT().Deploy(ctx),
				c.EXPECT().Get(gomock.AssignableToTypeOf(context.TODO()), kubernetesutils.Key(seedNamespace, "generic-token-kubeconfig"), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				// deploy Loki
				chartApplier.EXPECT().Apply(ctx, filepath.Join(ChartsPath, "seed-bootstrap", "charts", "loki"), seedNamespace, fmt.Sprintf("%s-logging", seedNamespace), gomock.AssignableToTypeOf(kubernetes.Values(map[string]interface{}{"Loki": "image"}))),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should not deploy event logger when it is disabled", func() {
			*botanist.Config.Logging.ShootEventLogging.Enabled = false
			gomock.InOrder(
				// destroy Shoot Event Logging
				shootEventLoggerDeployer.EXPECT().Destroy(ctx),
				// deploy Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Deploy(ctx),
				c.EXPECT().Get(gomock.AssignableToTypeOf(context.TODO()), kubernetesutils.Key(seedNamespace, "generic-token-kubeconfig"), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				// deploy Loki
				chartApplier.EXPECT().Apply(ctx, filepath.Join(ChartsPath, "seed-bootstrap", "charts", "loki"), seedNamespace, fmt.Sprintf("%s-logging", seedNamespace), gomock.AssignableToTypeOf(kubernetes.Values(map[string]interface{}{"Loki": "image"}))),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging when it is disabled", func() {
			botanist.Config.Logging.ShootNodeLogging = nil
			gomock.InOrder(
				// deploy Shoot Event Logging
				shootEventLoggerDeployer.EXPECT().Deploy(ctx),
				// destroy Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: seedNamespace}}),
				// deploy Loki
				chartApplier.EXPECT().Apply(ctx, filepath.Join(ChartsPath, "seed-bootstrap", "charts", "loki"), seedNamespace, fmt.Sprintf("%s-logging", seedNamespace), gomock.AssignableToTypeOf(kubernetes.Values(map[string]interface{}{"Loki": "image"}))),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		It("should not deploy shoot node logging and Loki when Loki is disabled", func() {
			*botanist.Config.Logging.Loki.Enabled = false
			gomock.InOrder(
				// deploy Shoot Event Logging
				shootEventLoggerDeployer.EXPECT().Deploy(ctx),
				// destroy Shoot Node Logging
				shootRBACProxyDeployer.EXPECT().Destroy(ctx),
				c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: seedNamespace}}),
				// destroy Loki
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
				c.EXPECT().Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: seedNamespace}}),
			)

			Expect(botanist.DeploySeedLogging(ctx)).To(Succeed())
		})

		Context("Tests expecting a failure", func() {
			It("should fail to delete the logging stack when ShootRBACProxyDeployer Destroy returns error", func() {
				*botanist.Config.Logging.Enabled = false
				shootRBACProxyDeployer.EXPECT().Destroy(ctx).Return(fakeErr)
				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to delete the logging stack when destroying the Shoot Node Logging fails", func() {
				botanist.Shoot.Purpose = shootPurposeTesting
				gomock.InOrder(
					// Destroying the Shoot Node Logging
					shootRBACProxyDeployer.EXPECT().Destroy(ctx),
					c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to delete the logging stack when ShootEventLoggerDeployer Destroy return error", func() {
				*botanist.Config.Logging.Enabled = false
				gomock.InOrder(
					// Destroying the Shoot Node Logging
					shootRBACProxyDeployer.EXPECT().Destroy(ctx),
					c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
					c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
					c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: seedNamespace}}),
					// Destroying the Shoot Event Logging
					shootEventLoggerDeployer.EXPECT().Destroy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should successfully delete the logging stack when shoot is with testing purpose", func() {
				*botanist.Config.Logging.Enabled = false
				gomock.InOrder(
					// Destroying the Shoot Node Logging
					shootRBACProxyDeployer.EXPECT().Destroy(ctx),
					c.EXPECT().Delete(ctx, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}),
					c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: seedNamespace}}),
					c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: seedNamespace}}),
					// Destroying the Shoot Event Logging
					shootEventLoggerDeployer.EXPECT().Destroy(ctx),
					// Delete Loki
					c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: seedNamespace}}),
					c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: seedNamespace}}),
					c.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: seedNamespace}}).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail when can't find Loki image", func() {
				botanist.Operation.ImageVector = imagevector.ImageVector{
					{Name: "loki-curator"},
					{Name: "kube-rbac-proxy"},
					{Name: "telegraf"},
				}

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail when can't find generic-token-kubeconfig", func() {
				gomock.InOrder(
					shootEventLoggerDeployer.EXPECT().Deploy(ctx),
					shootRBACProxyDeployer.EXPECT().Deploy(ctx),
					c.EXPECT().Get(gomock.AssignableToTypeOf(context.TODO()), kubernetesutils.Key(seedNamespace, "generic-token-kubeconfig"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(gr, "generic-token-kubeconfig")),
				)
				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when ShootEventLoggerDeployer Deploy returns an error", func() {
				gomock.InOrder(
					shootEventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when deploying of the shoot event logging fails", func() {
				gomock.InOrder(
					// deploy Shoot Event Logging
					shootEventLoggerDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when KubeRBACProxyDeployer Deploy returns an error", func() {
				gomock.InOrder(
					shootEventLoggerDeployer.EXPECT().Deploy(ctx),
					shootRBACProxyDeployer.EXPECT().Deploy(ctx).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack when generation of the Loki ingress TLS Secret fails", func() {
				gomock.InOrder(
					shootEventLoggerDeployer.EXPECT().Deploy(ctx),
					shootRBACProxyDeployer.EXPECT().Deploy(ctx),
					c.EXPECT().Get(gomock.AssignableToTypeOf(context.TODO()), kubernetesutils.Key(seedNamespace, "generic-token-kubeconfig"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})

			It("should fail to deploy the logging stack Loki charts failed to be applied", func() {
				gomock.InOrder(
					shootEventLoggerDeployer.EXPECT().Deploy(ctx),
					shootRBACProxyDeployer.EXPECT().Deploy(ctx),
					c.EXPECT().Get(gomock.AssignableToTypeOf(context.TODO()), kubernetesutils.Key(seedNamespace, "generic-token-kubeconfig"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					chartApplier.EXPECT().Apply(ctx, filepath.Join(ChartsPath, "seed-bootstrap", "charts", "loki"), seedNamespace, fmt.Sprintf("%s-logging", seedNamespace), gomock.AssignableToTypeOf(kubernetes.Values(map[string]interface{}{"Loki": "image"}))).Return(fakeErr),
				)

				Expect(botanist.DeploySeedLogging(ctx)).ToNot(Succeed())
			})
		})
	})
})
