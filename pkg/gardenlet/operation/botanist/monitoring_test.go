// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

		botanist              *Botanist
		alertManager          *mockalertmanager.MockInterface
		controlPlaneNamespace = "shoot--foo--bar"

		ingressAuthSecret     *corev1.Secret
		ingressWildcardSecret *corev1.Secret
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeSeedClient, controlPlaneNamespace)

		alertManager = mockalertmanager.NewMockInterface(ctrl)

		botanist = &Botanist{
			Operation: &operation.Operation{
				SecretsManager: fakeSecretManager,
				SeedClientSet:  kubernetesfake.NewClientSetBuilder().WithClient(fakeSeedClient).Build(),
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
					Purpose:               gardencorev1beta1.ShootPurposeProduction,
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							Alertmanager: alertManager,
						},
					},
					WantsAlertmanager: true,
				},
				ControlPlaneWildcardCert: ingressWildcardSecret,
			},
		}

		ingressAuthSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "observability-ingress-users", Namespace: controlPlaneNamespace}}
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
