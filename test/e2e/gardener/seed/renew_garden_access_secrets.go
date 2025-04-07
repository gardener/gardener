// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/managedseed"
)

var _ = Describe("Seed Tests", Label("Seed", "default"), func() {
	Describe("Garden Cluster Access For Seed Components", func() {
		var (
			seed             *gardencorev1beta1.Seed
			seedNamespace    string
			gardenAccessName string

			accessSecret *corev1.Secret
		)

		BeforeEach(func() {
			// Find the first seed which is not "e2e-managedseed". Seed name differs between test scenarios, e.g., non-ha/ha.
			// However, this test should not use "e2e-managedseed", because it is created and deleted in a separate e2e test.
			// Thus, it might be already gone before the garden cluster access was renewed.
			seedList := &gardencorev1beta1.SeedList{}
			Expect(testClient.List(ctx, seedList)).To(Succeed())
			for _, s := range seedList.Items {
				if s.Name != managedseed.GetSeedName() {
					seed = s.DeepCopy()
					break
				}
			}
			log.Info("Renewing garden cluster access", "seedName", seed.Name)

			seedNamespace = gardenerutils.ComputeGardenNamespace(seed.Name)

			gardenAccessName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

			accessSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenAccessName,
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
						resourcesv1alpha1.ResourceManagerClass:   resourcesv1alpha1.ResourceManagerClassGarden,
					},
					Annotations: map[string]string{
						resourcesv1alpha1.ServiceAccountName: gardenAccessName,
					},
				},
			}
		})

		It("should request tokens for garden access secrets", func() {
			By("Create garden access secret")
			Expect(testClient.Create(ctx, accessSecret)).To(Succeed())
			log.Info("Created garden access secret for test", "secret", client.ObjectKeyFromObject(accessSecret))

			DeferCleanup(func() {
				By("Delete garden access secret")
				Expect(testClient.Delete(ctx, accessSecret)).To(Succeed())
			})

			createRBACForGardenAccessServiceAccount(gardenAccessName, seedNamespace)

			By("Wait for token to be populated in garden access secret")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret)).To(Succeed())
				g.Expect(accessSecret.Data).To(HaveKeyWithValue(resourcesv1alpha1.DataKeyToken, Not(BeEmpty())))
			}).Should(Succeed())

			By("Use token to access garden")
			gardenAccessConfig := rest.CopyConfig(restConfig)
			// drop kind admin client certificate so that we can test other credentials
			gardenAccessConfig.CertData = nil
			gardenAccessConfig.KeyData = nil
			// use the requested token and create a client
			gardenAccessConfig.BearerToken = string(accessSecret.Data[resourcesv1alpha1.DataKeyToken])
			gardenAccessClient, err := client.New(gardenAccessConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(gardenAccessClient.Get(ctx, client.ObjectKey{Name: gardenAccessName, Namespace: seedNamespace}, &corev1.ServiceAccount{})).To(Succeed())
			}).Should(Succeed())
		})

		It("should renew all garden access secrets when triggered by annotation", func() {
			By("Create garden access secret")
			Expect(testClient.Create(ctx, accessSecret)).To(Succeed())
			log.Info("Created garden access secret for test", "secret", client.ObjectKeyFromObject(accessSecret))

			DeferCleanup(func() {
				By("Delete garden access secret")
				Expect(testClient.Delete(ctx, accessSecret)).To(Succeed())
			})

			By("Wait for token to be populated in garden access secret")
			var accessSecretBefore *corev1.Secret
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret)).To(Succeed())
				g.Expect(accessSecret.Data).To(HaveKeyWithValue(resourcesv1alpha1.DataKeyToken, Not(BeEmpty())))
				accessSecretBefore = accessSecret.DeepCopy()
			}).Should(Succeed())

			By("Trigger renewal of garden access secrets")
			patch := client.MergeFrom(seed.DeepCopy())
			metav1.SetMetaDataAnnotation(&seed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.SeedOperationRenewGardenAccessSecrets)
			Eventually(func() error {
				return testClient.Patch(ctx, seed, patch)
			}).Should(Succeed())

			By("Wait for operation annotation to be removed from Seed")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			}).Should(Succeed())

			By("Wait for token to be renewed in garden access secret")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret)).To(Succeed())
				g.Expect(accessSecret.Data).To(HaveKeyWithValue(resourcesv1alpha1.DataKeyToken, Not(Equal(accessSecretBefore.Data[resourcesv1alpha1.DataKeyToken]))))
				g.Expect(accessSecret.Annotations).To(HaveKeyWithValue(resourcesv1alpha1.ServiceAccountTokenRenewTimestamp, Not(Equal(accessSecretBefore.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]))))
			}).Should(Succeed())
		})

		Describe("usage in provider-local", func() {
			It("should be allowed via seed authorizer to annotate its own seed", func() {
				const testAnnotation = "provider-local-e2e-test-garden-access"

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())

					g.Expect(seed.Annotations).To(HaveKey(testAnnotation))
					g.Expect(time.Parse(time.RFC3339, seed.Annotations[testAnnotation])).
						Should(BeTemporally(">", seed.CreationTimestamp.UTC()),
							"Timestamp in %s annotation on seed %s should be after creationTimestamp of seed", testAnnotation, seed.Name)
				}).Should(Succeed())
			})
		})
	})
})

func createRBACForGardenAccessServiceAccount(name, namespace string) {
	By("Create RBAC resources for ServiceAccount")
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{""},
			Resources:     []string{"serviceaccounts"},
			Verbs:         []string{"get"},
			ResourceNames: []string{name},
		}},
	}
	Expect(testClient.Create(ctx, role)).To(Succeed())
	log.Info("Created role for test", "role", client.ObjectKeyFromObject(role))

	DeferCleanup(func() {
		Expect(testClient.Delete(ctx, role)).To(Succeed())
	})

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{{
			Kind: rbacv1.ServiceAccountKind,
			Name: name,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     name,
		},
	}
	Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	log.Info("Created role binding for test", "roleBinding", client.ObjectKeyFromObject(roleBinding))

	DeferCleanup(func() {
		Expect(testClient.Delete(ctx, roleBinding)).To(Succeed())
	})
}
