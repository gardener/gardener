// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package stale_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Project Stale controller tests", func() {
	var project *gardencorev1beta1.Project

	BeforeEach(func() {
		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-" + utils.ComputeSHA256Hex([]byte(testRunID + CurrentSpecReport().LeafNodeLocation.String()))[:5],
				Labels: map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &testNamespace.Name,
			},
		}

		By("Create Project")
		Expect(testClient.Create(ctx, project)).To(Succeed())
		log.Info("Created Project", "project", client.ObjectKeyFromObject(project))

		DeferCleanup(func() {
			By("Delete Project")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())

			By("Wait for Project to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
			}).Should(BeNotFoundError())
		})

		fakeClock.SetTime(time.Now())
	})

	Context("when project should be considered 'not stale'", func() {
		BeforeEach(func() {
			By("Mark the project as 'stale'")
			patch := client.MergeFrom(project.DeepCopy())
			project.Status.StaleSinceTimestamp = &metav1.Time{Time: fakeClock.Now()}
			Expect(testClient.Patch(ctx, project, patch)).To(Succeed())
		})

		AfterEach(func() {
			By("Wait for controller to mark project as 'not stale'")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				g.Expect(project.Status.StaleAutoDeleteTimestamp).To(BeNil())
			}).Should(Succeed())
		})

		It("project is too young", func() {
			fakeClock.Step(minimumLifetimeDays / 2 * 24 * time.Hour)
		})

		It("namespace has 'skip' annotation", func() {
			By("Add 'skip' annotation to namespace")
			patch := client.MergeFrom(testNamespace.DeepCopy())
			metav1.SetMetaDataAnnotation(&testNamespace.ObjectMeta, "project.gardener.cloud/skip-stale-check", "true")
			Expect(testClient.Patch(ctx, testNamespace, patch)).To(Succeed())

			DeferCleanup(func() {
				patch = client.MergeFrom(testNamespace.DeepCopy())
				delete(testNamespace.Annotations, "project.gardener.cloud/skip-stale-check")
				Expect(testClient.Patch(ctx, testNamespace, patch)).To(Succeed())
			})
		})

		It("project was recently active", func() {
			patch := client.MergeFrom(project.DeepCopy())
			project.Status.LastActivityTimestamp = &metav1.Time{Time: fakeClock.Now()}
			Expect(testClient.Patch(ctx, project, patch)).To(Succeed())
		})

		It("project in use by a Shoot", func() {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: ptr.To("mysecretbinding"),
					CloudProfileName:  ptr.To("cloudprofile1"),
					Region:            "europe-central-1",
					Provider: gardencorev1beta1.Provider{
						Type: "foo-provider",
						Workers: []gardencorev1beta1.Worker{
							{
								Name:    "cpu-worker",
								Minimum: 3,
								Maximum: 3,
								Machine: gardencorev1beta1.Machine{
									Type: "large",
								},
							},
						},
					},
					DNS: &gardencorev1beta1.DNS{
						Domain: ptr.To("some-domain.example.com"),
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.31.1",
					},
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To("foo-networking"),
					},
				},
			}

			Expect(testClient.Create(ctx, shoot)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, shoot)).To(Succeed())
			})
		})

		It("project in use by a BackupEntry", func() {
			backupEntry := &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: "foo",
				},
			}

			Expect(testClient.Create(ctx, backupEntry)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, backupEntry)).To(Succeed())
			})
		})

		It("project in use by a Secret through SecretBinding", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels: map[string]string{
						testID:                                   testRunID,
						"reference.gardener.cloud/secretbinding": "true",
					},
				},
			}

			Expect(testClient.Create(ctx, secret)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, secret)).To(Succeed())
			})

			secretBinding := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
				Provider: &gardencorev1beta1.SecretBindingProvider{
					Type: "provider",
				},
			}

			Expect(testClient.Create(ctx, secretBinding)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, secretBinding)).To(Succeed())
			})
		})

		It("project in use by a Secret through CredentialsBinding", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels: map[string]string{
						testID: testRunID,
						"reference.gardener.cloud/credentialsbinding": "true",
					},
				},
			}

			Expect(testClient.Create(ctx, secret)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, secret)).To(Succeed())
			})

			credentialsBinding := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secret.Name,
					Namespace:  secret.Namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: "provider",
				},
			}

			Expect(testClient.Create(ctx, credentialsBinding)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, credentialsBinding)).To(Succeed())
			})
		})

		It("project in use by a Quota", func() {
			quota := &gardencorev1beta1.Quota{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.QuotaSpec{
					Scope: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
					},
				},
			}

			Expect(testClient.Create(ctx, quota)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, quota)).To(Succeed())
			})

			secretBinding := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				SecretRef: corev1.SecretReference{
					Name:      "foo",
					Namespace: "foo",
				},
				Provider: &gardencorev1beta1.SecretBindingProvider{
					Type: "provider",
				},
				Quotas: []corev1.ObjectReference{{
					Name:      quota.Name,
					Namespace: quota.Namespace,
				}},
			}

			Expect(testClient.Create(ctx, secretBinding)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, secretBinding)).To(Succeed())
			})
		})
	})

	Context("when project should be considered 'stale'", func() {
		BeforeEach(func() {
			fakeClock.Step(minimumLifetimeDays * 24 * time.Hour)
		})

		It("project was active too long ago but grace period has not passed", func() {
			patch := client.MergeFrom(project.DeepCopy())
			project.Status.LastActivityTimestamp = &metav1.Time{Time: fakeClock.Now().Add(-minimumLifetimeDays * 24 * time.Hour)}
			Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

			By("Wait for controller to mark project as 'stale'")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleSinceTimestamp).NotTo(BeNil())
			}).Should(Succeed())

			By("Ensure auto-delete timestamp is not set")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleAutoDeleteTimestamp).To(BeNil())
			}).Should(Succeed())
		})

		It("project was active too long ago and grace period has passed", func() {
			patch := client.MergeFrom(project.DeepCopy())
			project.Status.LastActivityTimestamp = &metav1.Time{Time: fakeClock.Now().Add(-minimumLifetimeDays * 24 * time.Hour)}
			Expect(testClient.Patch(ctx, project, patch)).To(Succeed())

			By("Wait for controller to mark project as 'stale'")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleSinceTimestamp).NotTo(BeNil())
			}).Should(Succeed())

			fakeClock.Step(staleGracePeriodDays * 24 * time.Hour)

			By("Ensure auto-delete timestamp is set")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleAutoDeleteTimestamp).NotTo(BeNil())
				g.Expect(project.Status.StaleAutoDeleteTimestamp.UTC().Sub(project.Status.StaleSinceTimestamp.UTC())).To(Equal(staleExpirationTimeDays * 24 * time.Hour))
			}).Should(Succeed())
		})

		It("project is unused but not yet eligible for auto-deletion", func() {
			By("Wait for controller to mark project as 'stale'")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleSinceTimestamp).NotTo(BeNil())
			}).Should(Succeed())

			By("Ensure project is not deleted")
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
			}).Should(Succeed())
		})

		It("project is unused and eligible for auto-deletion", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleSinceTimestamp).NotTo(BeNil())
			}).Should(Succeed())

			fakeClock.Step(staleGracePeriodDays * 24 * time.Hour)

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
				g.Expect(project.Status.StaleAutoDeleteTimestamp).NotTo(BeNil())
			}).Should(Succeed())

			fakeClock.Step(staleExpirationTimeDays * 24 * time.Hour)

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
			}).Should(BeNotFoundError())
		})
	})
})
