// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	mockalertmanager "github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager/mock"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
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
		prometheus            *mockcomponent.MockWaiter
		controlPlaneNamespace = "shoot--foo--bar"

		ingressAuthSecret     *corev1.Secret
		ingressWildcardSecret *corev1.Secret
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeSeedClient, controlPlaneNamespace)

		alertManager = mockalertmanager.NewMockInterface(ctrl)
		prometheus = mockcomponent.NewMockWaiter(ctrl)

		botanist = &Botanist{
			Operation: &operation.Operation{
				SecretsManager: fakeSecretManager,
				SeedClientSet:  fakekubernetes.NewClientSetBuilder().WithClient(fakeSeedClient).Build(),
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
					Purpose:               gardencorev1beta1.ShootPurposeProduction,
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							Alertmanager: alertManager,
							Prometheus:   newMockPrometheus(prometheus),
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

	Describe("#WaitForAlertmanager", func() {
		It("should successfully wait for the alertmanager to become healthy", func() {
			alertManager.EXPECT().Wait(ctx).Return(nil)

			Expect(botanist.WaitForAlertManager(ctx)).To(Succeed())
		})

		It("should successfully wait for the alertmanager to be deleted", func() {
			botanist.Shoot.WantsAlertmanager = false

			alertManager.EXPECT().WaitCleanup(ctx).Return(nil)

			Expect(botanist.WaitForAlertManager(ctx)).To(Succeed())
		})
	})

	Describe("#WaitForPrometheus", func() {
		It("should successfully wait for prometheus", func() {
			prometheus.EXPECT().Wait(ctx)
			Expect(botanist.WaitForPrometheus(ctx)).To(Succeed())
		})

		It("should successfully wait for prometheus cleanup", func() {
			botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeTesting
			prometheus.EXPECT().WaitCleanup(ctx)
			Expect(botanist.WaitForPrometheus(ctx)).To(Succeed())
		})
	})
})

type mockPrometheus struct {
	prometheus.Interface

	waiter component.Waiter
}

func newMockPrometheus(w component.Waiter) prometheus.Interface {
	return &mockPrometheus{
		waiter: w,
	}
}

// SetIngressAuthSecret sets the ingress authentication secret name.
func (m *mockPrometheus) SetIngressAuthSecret(_ *corev1.Secret) {
	panic("unexpected call to SetIngressAuthSecret")
}

// SetIngressWildcardCertSecret sets the ingress wildcard certificate secret name.
func (m *mockPrometheus) SetIngressWildcardCertSecret(_ *corev1.Secret) {
	panic("unexpected call to SetIngressWildcardCertSecret")
}

// SetCentralScrapeConfigs sets the central scrape configs.
func (m *mockPrometheus) SetCentralScrapeConfigs(_ []*monitoringv1alpha1.ScrapeConfig) {
	panic("unexpected call to SetCentralScrapeConfigs")
}

// SetCentralPrometheusRules sets the central Prometheus rules.
func (m *mockPrometheus) SetCentralPrometheusRules(_ []*monitoringv1.PrometheusRule) {
	panic("unexpected call to SetCentralPrometheusRules")
}

// SetNamespaceUID sets the namespace UID.
func (m *mockPrometheus) SetNamespaceUID(_ types.UID) {
	panic("unexpected call to SetNamespaceUID")
}

// SetAdditionalAlertRelabelConfigs sets the additional alert relabel configs.
func (m *mockPrometheus) SetAdditionalAlertRelabelConfigs(_ []monitoringv1.RelabelConfig) {
	panic("unexpected call to SetAdditionalAlertRelabelConfigs")
}

// Deploy a component.
func (m *mockPrometheus) Deploy(_ context.Context) error {
	panic("unexpected call to Deploy")
}

// Destroy already deployed component.
func (m *mockPrometheus) Destroy(_ context.Context) error {
	panic("unexpected call to Destroy")
}

// Wait for deployment to finish and component to report ready.
func (m *mockPrometheus) Wait(ctx context.Context) error {
	return m.waiter.Wait(ctx)
}

// WaitCleanup for destruction to finish and component to be fully removed.
func (m *mockPrometheus) WaitCleanup(ctx context.Context) error {
	return m.waiter.WaitCleanup(ctx)
}
