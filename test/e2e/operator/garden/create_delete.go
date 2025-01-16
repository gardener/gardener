// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	. "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Garden Tests", Label("Garden", "default"), func() {
	var (
		backupSecret = defaultBackupSecret()
		garden       = defaultGarden(backupSecret, true)
	)

	It("Create, Delete", Label("simple"), func() {
		By("Create Garden")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()

		Expect(runtimeClient.Create(ctx, backupSecret)).To(Succeed())
		Expect(runtimeClient.Create(ctx, garden)).To(Succeed())

		waitForGardenToBeReconciled(ctx, garden)

		DeferCleanup(func() {
			ctx, cancel = context.WithTimeout(parentCtx, 5*time.Minute)
			defer cancel()

			By("Delete Garden")
			Expect(gardenerutils.ConfirmDeletion(ctx, runtimeClient, garden)).To(Succeed())
			Expect(runtimeClient.Delete(ctx, garden)).To(Succeed())
			Expect(runtimeClient.Delete(ctx, backupSecret)).To(Succeed())
			waitForGardenToBeDeleted(ctx, garden)
			cleanupVolumes(ctx)
			Expect(runtimeClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(namespace), client.MatchingLabels{"role": "kube-apiserver-etcd-encryption-configuration"})).To(Succeed())
			Expect(runtimeClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(namespace), client.MatchingLabels{"role": "gardener-apiserver-etcd-encryption-configuration"})).To(Succeed())

			By("Verify deletion")
			secretList := &corev1.SecretList{}
			Expect(runtimeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
				secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
				secretsmanager.LabelKeyManagerIdentity: operatorv1alpha1.SecretManagerIdentityOperator,
			})).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())

			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			Expect(runtimeClient.List(ctx, crdList)).To(Succeed())
			Expect(crdList.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gardens.operator.gardener.cloud")})})))

			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: namespace}, &appsv1.Deployment{})).To(BeNotFoundError())

			By("Wait until extension reports a successful uninstallation")
			waitForExtensionToReportDeletion(ctx, "provider-local")
		})

		By("Verify creation")
		CEventually(ctx, func(g Gomega) {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(runtimeClient.List(ctx, managedResourceList, client.InNamespace(namespace))).To(Succeed())
			g.Expect(managedResourceList.Items).To(ConsistOf(
				healthyManagedResource("vpa"),
				healthyManagedResource("etcd-druid"),
				healthyManagedResource("kube-state-metrics-runtime"),
				healthyManagedResource("shoot-core-kube-controller-manager"),
				healthyManagedResource("shoot-core-gardener-resource-manager"),
				healthyManagedResource("shoot-core-gardeneraccess"),
				healthyManagedResource("nginx-ingress"),
				healthyManagedResource("fluent-bit"),
				healthyManagedResource("fluent-operator"),
				healthyManagedResource("fluent-operator-custom-resources-garden"),
				healthyManagedResource("vali"),
				healthyManagedResource("plutono"),
				healthyManagedResource("prometheus-operator"),
				healthyManagedResource("alertmanager-garden"),
				healthyManagedResource("prometheus-garden"),
				healthyManagedResource("prometheus-garden-target"),
				healthyManagedResource("prometheus-longterm"),
				healthyManagedResource("blackbox-exporter"),
				healthyManagedResource("garden-system"),
				healthyManagedResource("garden-system-virtual"),
				healthyManagedResource("gardener-apiserver-runtime"),
				healthyManagedResource("gardener-apiserver-virtual"),
				healthyManagedResource("gardener-admission-controller-runtime"),
				healthyManagedResource("gardener-admission-controller-virtual"),
				healthyManagedResource("gardener-controller-manager-runtime"),
				healthyManagedResource("gardener-controller-manager-virtual"),
				healthyManagedResource("gardener-scheduler-runtime"),
				healthyManagedResource("gardener-scheduler-virtual"),
				healthyManagedResource("gardener-dashboard-runtime"),
				healthyManagedResource("gardener-dashboard-virtual"),
				healthyManagedResource("terminal-runtime"),
				healthyManagedResource("terminal-virtual"),
				healthyManagedResource("gardener-metrics-exporter-runtime"),
				healthyManagedResource("gardener-metrics-exporter-virtual"),
				healthyManagedResource("extension-admission-runtime-provider-local"),
				healthyManagedResource("extension-admission-virtual-provider-local"),
				healthyManagedResource("extension-registration-provider-local"),
				healthyManagedResource("extension-provider-local-garden"),
				healthyManagedResource("local-ext-shoot"),
			))

			g.Expect(runtimeClient.List(ctx, managedResourceList, client.InNamespace("istio-system"))).To(Succeed())
			g.Expect(managedResourceList.Items).To(ConsistOf(
				healthyManagedResource("istio-system"),
				healthyManagedResource("virtual-garden-istio"),
			))
		}).WithPolling(2 * time.Second).Should(Succeed())

		var virtualClusterClient kubernetes.Interface
		By("Verify virtual cluster access using token-request kubeconfig")
		Eventually(func(g Gomega) {
			var err error
			virtualClusterClient, err = kubernetes.NewClientFromSecret(ctx, runtimeClient, namespace, "gardener",
				kubernetes.WithDisabledCachedClient(),
				kubernetes.WithClientOptions(client.Options{Scheme: operatorclient.VirtualScheme}),
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(virtualClusterClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())

		By("Verify Gardener APIs availability")
		Eventually(func(g Gomega) {
			g.Expect(virtualClusterClient.Client().List(ctx, &gardencorev1beta1.ShootList{})).To(Succeed())
			g.Expect(virtualClusterClient.Client().List(ctx, &seedmanagementv1alpha1.ManagedSeedList{})).To(Succeed())
			g.Expect(virtualClusterClient.Client().List(ctx, &settingsv1alpha1.ClusterOpenIDConnectPresetList{})).To(Succeed())
			g.Expect(virtualClusterClient.Client().List(ctx, &operationsv1alpha1.BastionList{})).To(Succeed())
		}).Should(Succeed())

		By("Verify virtual cluster extension installations")
		Eventually(func(g Gomega) {
			controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
			g.Expect(virtualClusterClient.Client().List(ctx, controllerRegistrationList)).To(Succeed())
			g.Expect(controllerRegistrationList.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("provider-local")})})))
			controllerDeploymentList := &gardencorev1.ControllerDeploymentList{}
			g.Expect(virtualClusterClient.Client().List(ctx, controllerDeploymentList)).To(Succeed())
			g.Expect(controllerDeploymentList.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("provider-local")})})))
		}).Should(Succeed())
	})
})

func healthyManagedResource(name string) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal(name)}),
		"Status": MatchFields(IgnoreExtras, Fields{"Conditions": And(
			ContainCondition(OfType(resourcesv1alpha1.ResourcesApplied), WithStatus(gardencorev1beta1.ConditionTrue)),
			ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue)),
			ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse)),
		)}),
	})
}
