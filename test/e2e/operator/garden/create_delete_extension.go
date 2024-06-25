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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Garden Tests", Label("Garden", "default"), func() {
	var (
		backupSecret = defaultBackupSecret()
		garden       = defaultGarden(backupSecret)
		extension    = defaultExtension()
	)

	It("Create, Delete", Label("extension"), func() {
		By("Create Garden")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()

		Expect(client.IgnoreAlreadyExists(runtimeClient.Create(ctx, backupSecret))).To(Succeed())
		Expect(client.IgnoreAlreadyExists(runtimeClient.Create(ctx, extension))).To(Succeed())
		Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
		waitForGardenToBeReconciled(ctx, garden)

		DeferCleanup(func() {
			By("Delete Garden")
			ctx, cancel = context.WithTimeout(parentCtx, 5*time.Minute)
			defer cancel()

			Expect(gardenerutils.ConfirmDeletion(ctx, runtimeClient, garden)).To(Succeed())
			Expect(runtimeClient.Delete(ctx, garden)).To(Succeed())
			Expect(runtimeClient.Delete(ctx, backupSecret)).To(Succeed())
			waitForGardenToBeDeleted(ctx, garden)
			waitForExtensionToBeDeleted(ctx, extension)
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
		})

		By("Verify creation")
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

		By("Issue extension deletion")
		Expect(runtimeClient.Delete(ctx, extension)).To(Succeed())
		By("Verify virtual cluster controller-registration and controller-deployment deletion")
		Expect(runtimeClient.Delete(ctx, extension)).To(Succeed())
		Eventually(func(g Gomega) {
			var ctrlDep gardencorev1beta1.ControllerDeployment
			var ctrlReg gardencorev1beta1.ControllerRegistration
			g.Expect(virtualClusterClient.Client().Get(ctx, client.ObjectKey{Name: extensionName}, &ctrlDep)).To(BeNotFoundError())
			g.Expect(virtualClusterClient.Client().Get(ctx, client.ObjectKey{Name: extensionName}, &ctrlReg)).To(BeNotFoundError())
		}).Should(Succeed())
	})
})
