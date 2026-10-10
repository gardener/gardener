// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package refallowlist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed Credentials controller", func() {
	var (
		backupSecret      *corev1.Secret
		dnsProviderSecret *corev1.Secret
		dnsInternalWI     *securityv1alpha1.WorkloadIdentity
		dnsDefaultSecret  *corev1.Secret
		resourceCM        *corev1.ConfigMap
		config            *gardenletconfigv1alpha1.GardenletConfiguration
		seedSpec          gardencorev1beta1.SeedSpec
	)

	BeforeEach(func() {
		backupSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "backup-creds-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
		By("Create backup Secret")
		Expect(testClient.Create(ctx, backupSecret)).To(Succeed())
		log.Info("Created backup Secret for test", "secret", client.ObjectKeyFromObject(backupSecret))
		DeferCleanup(func() {
			By("Delete backup Secret")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, backupSecret))).To(Succeed())
		})

		dnsProviderSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "dns-provider-creds-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
		By("Create DNS provider Secret")
		Expect(testClient.Create(ctx, dnsProviderSecret)).To(Succeed())
		log.Info("Created DNS provider Secret for test", "secret", client.ObjectKeyFromObject(dnsProviderSecret))
		DeferCleanup(func() {
			By("Delete DNS provider Secret")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, dnsProviderSecret))).To(Succeed())
		})

		dnsInternalWI = &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "dns-internal-wi-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: securityv1alpha1.WorkloadIdentitySpec{
				Audiences:    []string{"aud"},
				TargetSystem: securityv1alpha1.TargetSystem{Type: "test"},
			},
		}
		By("Create DNS internal WorkloadIdentity")
		Expect(testClient.Create(ctx, dnsInternalWI)).To(Succeed())
		log.Info("Created DNS internal WorkloadIdentity for test", "workloadIdentity", client.ObjectKeyFromObject(dnsInternalWI))
		DeferCleanup(func() {
			By("Delete DNS internal WorkloadIdentity")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, dnsInternalWI))).To(Succeed())
		})

		dnsDefaultSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "dns-default-creds-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
		By("Create DNS default Secret")
		Expect(testClient.Create(ctx, dnsDefaultSecret)).To(Succeed())
		log.Info("Created DNS default Secret for test", "secret", client.ObjectKeyFromObject(dnsDefaultSecret))
		DeferCleanup(func() {
			By("Delete DNS default Secret")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, dnsDefaultSecret))).To(Succeed())
		})

		resourceCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "resource-cm-",
				Namespace:    gardenNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
		By("Create resource ConfigMap")
		Expect(testClient.Create(ctx, resourceCM)).To(Succeed())
		log.Info("Created resource ConfigMap for test", "configMap", client.ObjectKeyFromObject(resourceCM))
		DeferCleanup(func() {
			By("Delete resource ConfigMap")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceCM))).To(Succeed())
		})

		seedSpec = gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{Type: "test"},
			Backup: &gardencorev1beta1.Backup{
				Provider: "test",
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  testNamespace.Name,
					Name:       backupSecret.Name,
				},
			},
			DNS: gardencorev1beta1.SeedDNS{
				Provider: &gardencorev1beta1.SeedDNSProvider{
					Type: "test",
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  testNamespace.Name,
						Name:       dnsProviderSecret.Name,
					},
				},
				Internal: &gardencorev1beta1.SeedDNSProviderConfig{
					Type:   "test",
					Domain: "internal.example.com",
					CredentialsRef: corev1.ObjectReference{
						APIVersion: "security.gardener.cloud/v1alpha1",
						Kind:       "WorkloadIdentity",
						Namespace:  testNamespace.Name,
						Name:       dnsInternalWI.Name,
					},
				},
				Defaults: []gardencorev1beta1.SeedDNSProviderConfig{
					{
						Type:   "test",
						Domain: "default.example.com",
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  testNamespace.Name,
							Name:       dnsDefaultSecret.Name,
						},
					},
				},
			},
			Resources: []gardencorev1beta1.NamedResourceReference{
				{
					Name: "my-resource",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       resourceCM.Name,
					},
				},
			},
		}

		config = &gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					Spec: seedSpec,
				},
			},
		}
	})

	encodeConfig := func(cfg *gardenletconfigv1alpha1.GardenletConfiguration) runtime.RawExtension {
		raw, err := encoding.EncodeGardenletConfiguration(cfg)
		Expect(err).NotTo(HaveOccurred())
		return *raw
	}

	expectAnnotated := func(g Gomega, seedName string) {
		for _, obj := range []client.Object{backupSecret, dnsProviderSecret, dnsInternalWI, dnsDefaultSecret, resourceCM} {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed(), "get %s", client.ObjectKeyFromObject(obj))
			g.Expect(obj.GetAnnotations()).To(HaveKeyWithValue("seed.gardener.cloud/names", seedName), "object %s", client.ObjectKeyFromObject(obj))
		}
	}

	expectNotAnnotated := func(g Gomega) {
		for _, obj := range []client.Object{backupSecret, dnsProviderSecret, dnsInternalWI, dnsDefaultSecret, resourceCM} {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed(), "get %s", client.ObjectKeyFromObject(obj))
			g.Expect(obj.GetAnnotations()).NotTo(HaveKey("seed.gardener.cloud/names"), "object %s", client.ObjectKeyFromObject(obj))
		}
	}

	Describe("Gardenlet", func() {
		var gardenlet *seedmanagementv1alpha1.Gardenlet

		BeforeEach(func() {
			gardenlet = &seedmanagementv1alpha1.Gardenlet{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "seed-",
					Namespace:    gardenNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: seedmanagementv1alpha1.GardenletSpec{
					Deployment: seedmanagementv1alpha1.GardenletSelfDeployment{
						GardenletDeployment: seedmanagementv1alpha1.GardenletDeployment{},
						Helm: seedmanagementv1alpha1.GardenletHelm{
							OCIRepository: gardencorev1.OCIRepository{
								Repository: new("europe-docker.pkg.dev/gardener-project/releases/charts/gardener/gardenlet"),
								Tag:        new("v0.0.1"),
							},
						},
					},
					Config: encodeConfig(config),
				},
			}
		})

		JustBeforeEach(func() {
			By("Create Gardenlet")
			Expect(testClient.Create(ctx, gardenlet)).To(Succeed())
			log.Info("Created Gardenlet for test", "gardenlet", client.ObjectKeyFromObject(gardenlet))
			DeferCleanup(func() {
				By("Delete Gardenlet")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, gardenlet))).To(Succeed())
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)
				}).Should(BeNotFoundError())
			})
		})

		It("should stamp the annotation on all referenced resources", func() {
			Eventually(func(g Gomega) {
				expectAnnotated(g, gardenlet.Name)
			}).Should(Succeed())
		})

		It("should remove the annotation on Gardenlet deletion", func() {
			By("Wait for annotations to be stamped")
			Eventually(func(g Gomega) {
				expectAnnotated(g, gardenlet.Name)
			}).Should(Succeed())

			By("Delete Gardenlet")
			Expect(testClient.Delete(ctx, gardenlet)).To(Succeed())

			Eventually(func(g Gomega) {
				expectNotAnnotated(g)
			}).Should(Succeed())
		})
	})

	Describe("ManagedSeed", func() {
		var managedSeed *seedmanagementv1alpha1.ManagedSeed

		BeforeEach(func() {
			managedSeed = &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "seed-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: seedmanagementv1alpha1.ManagedSeedSpec{
					Shoot: &seedmanagementv1alpha1.Shoot{Name: "my-shoot"},
					Gardenlet: seedmanagementv1alpha1.GardenletConfig{
						Config: encodeConfig(config),
					},
				},
			}
		})

		JustBeforeEach(func() {
			By("Create ManagedSeed")
			Expect(testClient.Create(ctx, managedSeed)).To(Succeed())
			log.Info("Created ManagedSeed for test", "managedSeed", client.ObjectKeyFromObject(managedSeed))
			DeferCleanup(func() {
				By("Delete ManagedSeed")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, managedSeed))).To(Succeed())
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)
				}).Should(BeNotFoundError())
			})
		})

		It("should stamp the annotation on all referenced resources", func() {
			Eventually(func(g Gomega) {
				expectAnnotated(g, managedSeed.Name)
			}).Should(Succeed())
		})

		It("should remove the annotation on ManagedSeed deletion", func() {
			By("Wait for annotations to be stamped")
			Eventually(func(g Gomega) {
				expectAnnotated(g, managedSeed.Name)
			}).Should(Succeed())

			By("Delete ManagedSeed")
			Expect(testClient.Delete(ctx, managedSeed)).To(Succeed())

			Eventually(func(g Gomega) {
				expectNotAnnotated(g)
			}).Should(Succeed())
		})
	})
})
