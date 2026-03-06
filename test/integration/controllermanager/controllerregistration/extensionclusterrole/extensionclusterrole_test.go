// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionclusterrole_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ExtensionClusterRole controller tests", func() {
	var (
		seedNamespace1                *corev1.Namespace
		serviceAccount1SeedNamespace2 *corev1.ServiceAccount
		serviceAccount2SeedNamespace2 *corev1.ServiceAccount

		seedNamespace2                *corev1.Namespace
		serviceAccount1SeedNamespace1 *corev1.ServiceAccount
		serviceAccount2SeedNamespace1 *corev1.ServiceAccount

		nonSeedNamespace                *corev1.Namespace
		serviceAccount1NonSeedNamespace *corev1.ServiceAccount

		extensionSAGardenNamespace *corev1.ServiceAccount

		projectNamespace               *corev1.Namespace
		extensionSAProjectNamespace    *corev1.ServiceAccount
		nonExtensionSAProjectNamespace *corev1.ServiceAccount

		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	BeforeEach(func() {
		seedNamespace1 = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "seed-bar",
				Labels: map[string]string{testID: testRunID, "gardener.cloud/role": "seed"},
			},
		}
		serviceAccount1SeedNamespace1 = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-account1",
				Namespace: seedNamespace1.Name,
				Labels:    map[string]string{testID: testRunID, "relevant": "true"},
			},
		}
		serviceAccount2SeedNamespace1 = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-account2",
				Namespace: seedNamespace1.Name,
				Labels:    map[string]string{testID: testRunID, "relevant": "true"},
			},
		}

		By("Create Seed Namespace 1 and ServiceAccounts")
		Expect(testClient.Create(ctx, seedNamespace1)).To(Succeed())
		log.Info("Created Namespace for test", "namespace", client.ObjectKeyFromObject(seedNamespace1))

		Expect(testClient.Create(ctx, serviceAccount1SeedNamespace1)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(serviceAccount1SeedNamespace1))

		Expect(testClient.Create(ctx, serviceAccount2SeedNamespace1)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(serviceAccount2SeedNamespace1))

		DeferCleanup(func() {
			By("Delete ServiceAccounts and Seed Namespace 1")
			Expect(testClient.Delete(ctx, serviceAccount1SeedNamespace1)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, serviceAccount2SeedNamespace1)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, seedNamespace1)).To(Or(Succeed(), BeNotFoundError()))

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(seedNamespace1), &corev1.Namespace{})
			}).Should(BeNotFoundError())
		})

		seedNamespace2 = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "seed-foo",
				Labels: map[string]string{testID: testRunID, "gardener.cloud/role": "seed"},
			},
		}
		serviceAccount1SeedNamespace2 = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-account1",
				Namespace: seedNamespace2.Name,
				Labels:    map[string]string{testID: testRunID, "relevant": "true"},
			},
		}
		serviceAccount2SeedNamespace2 = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: seedNamespace2.Name,
				Labels:    map[string]string{testID: testRunID},
			},
		}

		By("Create Seed Namespace 2 and ServiceAccounts")
		Expect(testClient.Create(ctx, seedNamespace2)).To(Succeed())
		log.Info("Created Namespace for test", "namespace", client.ObjectKeyFromObject(seedNamespace2))

		Expect(testClient.Create(ctx, serviceAccount1SeedNamespace2)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(serviceAccount1SeedNamespace2))

		Expect(testClient.Create(ctx, serviceAccount2SeedNamespace2)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(serviceAccount2SeedNamespace2))

		DeferCleanup(func() {
			By("Delete ServiceAccounts and Seed Namespace 2")
			Expect(testClient.Delete(ctx, serviceAccount1SeedNamespace2)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, serviceAccount2SeedNamespace2)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, seedNamespace2)).To(Or(Succeed(), BeNotFoundError()))

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(seedNamespace2), &corev1.Namespace{})
			}).Should(BeNotFoundError())
		})

		nonSeedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-foo",
				Labels: map[string]string{testID: testRunID},
			},
		}
		serviceAccount1NonSeedNamespace = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-account1",
				Namespace: nonSeedNamespace.Name,
				Labels:    map[string]string{testID: testRunID, "relevant": "true"},
			},
		}

		By("Create non-Seed Namespace and ServiceAccount")
		Expect(testClient.Create(ctx, nonSeedNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespace", client.ObjectKeyFromObject(nonSeedNamespace))

		Expect(testClient.Create(ctx, serviceAccount1NonSeedNamespace)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(serviceAccount1NonSeedNamespace))

		DeferCleanup(func() {
			By("Delete ServiceAccount and non-Seed Namespace")
			Expect(testClient.Delete(ctx, serviceAccount1NonSeedNamespace)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, nonSeedNamespace)).To(Or(Succeed(), BeNotFoundError()))

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(nonSeedNamespace), &corev1.Namespace{})
			}).Should(BeNotFoundError())
		})

		// Extension SA in the garden namespace (not garden-* project namespace).
		extensionSAGardenNamespace = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "extension-shoot--my-shoot--foo",
				Namespace: "garden",
				Labels:    map[string]string{testID: testRunID, "relevant": "true"},
			},
		}

		By("Create extension ServiceAccount in garden namespace")
		Expect(testClient.Create(ctx, extensionSAGardenNamespace)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(extensionSAGardenNamespace))

		DeferCleanup(func() {
			By("Delete extension ServiceAccount from garden namespace")
			Expect(testClient.Delete(ctx, extensionSAGardenNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		projectNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-my-project",
				Labels: map[string]string{testID: testRunID},
			},
		}
		// Extension SA: prefixed with "extension-shoot--", should be included in binding subjects.
		extensionSAProjectNamespace = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "extension-shoot--my-shoot--foo",
				Namespace: projectNamespace.Name,
				Labels:    map[string]string{testID: testRunID, "relevant": "true"},
			},
		}
		// Non-extension SA: no "extension-shoot--" prefix, must be excluded even though labels match.
		nonExtensionSAProjectNamespace = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-extension-sa",
				Namespace: projectNamespace.Name,
				Labels:    map[string]string{testID: testRunID, "relevant": "true"},
			},
		}

		By("Create project Namespace and ServiceAccounts")
		Expect(testClient.Create(ctx, projectNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespace", client.ObjectKeyFromObject(projectNamespace))

		Expect(testClient.Create(ctx, extensionSAProjectNamespace)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(extensionSAProjectNamespace))

		Expect(testClient.Create(ctx, nonExtensionSAProjectNamespace)).To(Succeed())
		log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(nonExtensionSAProjectNamespace))

		DeferCleanup(func() {
			By("Delete ServiceAccounts and project Namespace")
			Expect(testClient.Delete(ctx, extensionSAProjectNamespace)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, nonExtensionSAProjectNamespace)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, projectNamespace)).To(Or(Succeed(), BeNotFoundError()))

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(projectNamespace), &corev1.Namespace{})
			}).Should(BeNotFoundError())
		})

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: testRunID,
				Annotations: map[string]string{
					"authorization.gardener.cloud/extensions-serviceaccount-selector": `{"matchLabels":{"relevant":"true"}}`,
				},
				Labels: map[string]string{
					"authorization.gardener.cloud/custom-extensions-permissions": "true",
					testID: testRunID,
				},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list"},
			}},
		}

		By("Create ClusterRole")
		Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
		log.Info("Created ClusterRole for test", "clusterRole", client.ObjectKeyFromObject(clusterRole))

		By("Wait until manager has observed clusterRole")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(clusterRole), &rbacv1.ClusterRole{})
		}).Should(Succeed())

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRole.Name}}

		DeferCleanup(func() {
			By("Delete ClusterRole")
			Expect(testClient.Delete(ctx, clusterRole)).To(Or(Succeed(), BeNotFoundError()))

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(clusterRole), &rbacv1.ClusterRole{})
			}).Should(BeNotFoundError())

			By("Delete ClusterRoleBinding")
			Expect(testClient.Delete(ctx, clusterRoleBinding)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), &rbacv1.ClusterRoleBinding{})
			}).Should(BeNotFoundError())
		})
	})

	It("should create the expected ClusterRoleBinding", func() {
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)
		}).Should(Succeed())

		Expect(clusterRoleBinding.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
			APIVersion:         "rbac.authorization.k8s.io/v1",
			Kind:               "ClusterRole",
			Name:               clusterRole.Name,
			UID:                clusterRole.UID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(false),
		}))
		Expect(clusterRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		}))
		Expect(clusterRoleBinding.Subjects).To(HaveExactElements(
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      extensionSAGardenNamespace.Name,
				Namespace: extensionSAGardenNamespace.Namespace,
			},
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      extensionSAProjectNamespace.Name,
				Namespace: projectNamespace.Name,
			},
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount1SeedNamespace1.Name,
				Namespace: seedNamespace1.Name,
			},
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount2SeedNamespace1.Name,
				Namespace: seedNamespace1.Name,
			},
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount1SeedNamespace2.Name,
				Namespace: seedNamespace2.Name,
			},
		))
	})

	When("a ServiceAccount is added or deleted", func() {
		It("should adjust the subjects", func() {
			serviceAccount3SeedNamespace1 := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new",
					Namespace: seedNamespace1.Name,
					Labels:    map[string]string{testID: testRunID, "relevant": "true"},
				},
			}

			By("Create ServiceAccount")
			Expect(testClient.Create(ctx, serviceAccount3SeedNamespace1)).To(Succeed())
			log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(serviceAccount3SeedNamespace1))

			By("Wait until manager has observed serviceAccount")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount3SeedNamespace1), &corev1.ServiceAccount{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete ServiceAccount")
				Expect(testClient.Delete(ctx, serviceAccount3SeedNamespace1)).To(Succeed())
			})

			Eventually(func(g Gomega) []rbacv1.Subject {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).To(Succeed())
				return clusterRoleBinding.Subjects
			}).Should(HaveExactElements(
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      extensionSAGardenNamespace.Name,
					Namespace: extensionSAGardenNamespace.Namespace,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      extensionSAProjectNamespace.Name,
					Namespace: projectNamespace.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount3SeedNamespace1.Name,
					Namespace: seedNamespace1.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount1SeedNamespace1.Name,
					Namespace: seedNamespace1.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount2SeedNamespace1.Name,
					Namespace: seedNamespace1.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount1SeedNamespace2.Name,
					Namespace: seedNamespace2.Name,
				},
			))

			By("Delete ServiceAccount")
			Expect(testClient.Delete(ctx, serviceAccount1SeedNamespace1)).To(Succeed())

			Eventually(func(g Gomega) []rbacv1.Subject {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).To(Succeed())
				return clusterRoleBinding.Subjects
			}).Should(HaveExactElements(
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      extensionSAGardenNamespace.Name,
					Namespace: extensionSAGardenNamespace.Namespace,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      extensionSAProjectNamespace.Name,
					Namespace: projectNamespace.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount3SeedNamespace1.Name,
					Namespace: seedNamespace1.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount2SeedNamespace1.Name,
					Namespace: seedNamespace1.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount1SeedNamespace2.Name,
					Namespace: seedNamespace2.Name,
				},
			))
		})
	})

	When("the label selector is changed", func() {
		It("should adjust the subjects", func() {
			// Wait till the clusterRoleBinding is created for the first time since we are calling Get for this object in Consistently right after creating the ServiceAccount
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)
			}).Should(Succeed())

			serviceAccount3SeedNamespace2 := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hb23b",
					Namespace: seedNamespace2.Name,
					Labels:    map[string]string{testID: testRunID, "new-relevant": "true"},
				},
			}

			By("Create ServiceAccount")
			Expect(testClient.Create(ctx, serviceAccount3SeedNamespace2)).To(Succeed())
			log.Info("Created ServiceAccount for test", "serviceAccount", client.ObjectKeyFromObject(serviceAccount3SeedNamespace2))

			By("Wait until manager has observed serviceAccount")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount3SeedNamespace2), &corev1.ServiceAccount{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete ServiceAccount")
				Expect(testClient.Delete(ctx, serviceAccount3SeedNamespace2)).To(Succeed())
			})

			By("Subjects should not change yet")
			Consistently(func(g Gomega) []rbacv1.Subject {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).To(Succeed())
				return clusterRoleBinding.Subjects
			}).Should(HaveExactElements(
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      extensionSAGardenNamespace.Name,
					Namespace: extensionSAGardenNamespace.Namespace,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      extensionSAProjectNamespace.Name,
					Namespace: projectNamespace.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount1SeedNamespace1.Name,
					Namespace: seedNamespace1.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount2SeedNamespace1.Name,
					Namespace: seedNamespace1.Name,
				},
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount1SeedNamespace2.Name,
					Namespace: seedNamespace2.Name,
				},
			))

			By("Patch ClusterRole")
			patch := client.MergeFrom(clusterRole.DeepCopy())
			clusterRole.Annotations["authorization.gardener.cloud/extensions-serviceaccount-selector"] = `{"matchLabels":{"new-relevant":"true"}}`
			Expect(testClient.Patch(ctx, clusterRole, patch)).To(Succeed())

			By("Subjects should include the service account of newly added extension")
			Eventually(func(g Gomega) []rbacv1.Subject {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).To(Succeed())
				return clusterRoleBinding.Subjects
			}).Should(HaveExactElements(
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount3SeedNamespace2.Name,
					Namespace: seedNamespace2.Name,
				},
			))
		})
	})
})
