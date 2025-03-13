// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/operator/garden/internal"
)

var _ = Describe("Garden Tests", Label("Garden", "default"), func() {
	Describe("Create, Delete Garden", Label("simple"), Ordered, func() {
		var s *GardenContext
		BeforeTestSetup(func() {
			backupSecret := defaultBackupSecret()
			s = NewTestContext().ForGarden(defaultGarden(backupSecret, true), backupSecret)
		})

		ItShouldCreateGarden(s)
		ItShouldWaitForGardenToBeReconciledAndHealthy(s)
		ItShouldVerifyGardenManagedResourcesAndAwaitHealthiness(s)
		ItShouldVerifyIstioManagedResourcesAndAwaitHealthiness(s)
		ItShouldInitializeVirtualClusterClient(s)

		It("Verify Gardener APIs availability", func(ctx SpecContext) {
			Eventually(ctx, s.VirtualClusterKomega.List(&gardencorev1beta1.ShootList{})).Should(Succeed())
			Eventually(ctx, s.VirtualClusterKomega.List(&seedmanagementv1alpha1.ManagedSeedList{})).Should(Succeed())
			Eventually(ctx, s.VirtualClusterKomega.List(&settingsv1alpha1.ClusterOpenIDConnectPresetList{})).Should(Succeed())
			Eventually(ctx, s.VirtualClusterKomega.List(&operationsv1alpha1.BastionList{})).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Verify virtual cluster extension installations", func(ctx SpecContext) {
			controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
			Eventually(ctx, s.VirtualClusterKomega.List(controllerRegistrationList)).Should(Succeed())
			Expect(controllerRegistrationList.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("provider-local")})})))

			controllerDeploymentList := &gardencorev1.ControllerDeploymentList{}
			Eventually(ctx, s.VirtualClusterKomega.List(controllerDeploymentList)).Should(Succeed())
			Expect(controllerDeploymentList.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("provider-local")})})))
		}, SpecTimeout(time.Minute))

		It("Verify 'gardener-system-public' namespace and 'gardener-info' configmap exist", func(ctx SpecContext) {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardencorev1beta1.GardenerSystemPublicNamespace,
				},
			}
			Eventually(ctx, s.VirtualClusterKomega.Get(namespace)).Should(Succeed())

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-info",
					Namespace: gardencorev1beta1.GardenerSystemPublicNamespace,
				},
			}
			Eventually(ctx, s.VirtualClusterKomega.Get(configMap)).Should(Succeed())
			Expect(configMap.Data).To(HaveKey("gardenerAPIServer"))
		}, SpecTimeout(time.Minute))

		ItShouldDeleteGarden(s)
		ItShouldWaitForGardenToBeDeleted(s)
		ItShouldCleanUp(s)

		It("Verify no leftover secrets", func(ctx SpecContext) {
			secretList := &corev1.SecretList{}
			Eventually(ctx, s.GardenKomega.List(secretList, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{
				secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
				secretsmanager.LabelKeyManagerIdentity: operatorv1alpha1.SecretManagerIdentityOperator,
			})).Should(Succeed())

			Expect(secretList.Items).To(BeEmpty())
		}, SpecTimeout(time.Minute))

		It("Verify CRD still present", func(ctx SpecContext) {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			Eventually(ctx, s.GardenKomega.List(crdList)).Should(Succeed())
			Expect(crdList.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardens.operator.gardener.cloud")})})))
		}, SpecTimeout(time.Minute))

		It("Verify gardener-resource-manager was deleted", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
					Namespace: v1beta1constants.GardenNamespace,
				},
			}

			Eventually(ctx, s.GardenKomega.Get(deployment)).Should(BeNotFoundError())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForExtensionToReportDeletion(s, "provider-local")

	})
})
