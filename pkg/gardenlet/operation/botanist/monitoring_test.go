// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockalertmanager "github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Monitoring", func() {
	var (
		ctx  = context.TODO()
		ctrl *gomock.Controller

		fakeSeedClient    client.Client
		fakeSecretManager secretsmanager.Interface

		botanist      *Botanist
		alertManager  *mockalertmanager.MockInterface
		seedNamespace = "shoot--foo--bar"

		ingressAuthSecret     *corev1.Secret
		ingressWildcardSecret *corev1.Secret
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeSeedClient, seedNamespace)

		alertManager = mockalertmanager.NewMockInterface(ctrl)

		botanist = &Botanist{
			Operation: &operation.Operation{
				SecretsManager: fakeSecretManager,
				SeedClientSet:  kubernetesfake.NewClientSetBuilder().WithClient(fakeSeedClient).Build(),
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
					Purpose:       gardencorev1beta1.ShootPurposeProduction,
					Components: &shootpkg.Components{
						Monitoring: &shootpkg.Monitoring{
							Alertmanager: alertManager,
						},
					},
					WantsAlertmanager: true,
				},
				ControlPlaneWildcardCert: ingressWildcardSecret,
			},
		}

		ingressAuthSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "observability-ingress-users", Namespace: seedNamespace}}
		ingressWildcardSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wildcard"}}

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeSeedClient.Create(ctx, ingressAuthSecret)).To(Succeed())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployAlertmanager", func() {
		Context("deletion", func() {
			It("should successfully delete the alertmanager when not wanted", func() {
				botanist.Shoot.WantsAlertmanager = false

				alertManager.EXPECT().Destroy(ctx)

				Expect(botanist.DeployAlertManager(ctx)).To(Succeed())
			})

			It("should successfully delete the alertmanager when shoot monitoring disabled", func() {
				botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeTesting

				alertManager.EXPECT().Destroy(ctx)

				Expect(botanist.DeployAlertManager(ctx)).To(Succeed())
			})
		})

		Context("deployment", func() {
			It("should successfully deploy", func() {
				alertManager.EXPECT().SetIngressAuthSecret(gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(s *corev1.Secret) {
					Expect(s.Name).To(Equal(ingressAuthSecret.Name))
				})
				alertManager.EXPECT().SetIngressWildcardCertSecret(gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(s *corev1.Secret) {
					Expect(s.Name).To(Equal(ingressWildcardSecret.Name))
				})
				alertManager.EXPECT().Deploy(ctx)

				Expect(botanist.DeployAlertManager(ctx)).To(Succeed())
			})
		})
	})
})
