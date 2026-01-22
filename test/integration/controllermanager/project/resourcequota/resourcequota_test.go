// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota_test

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/resourcequota"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	ActualConfigMaps = 2
	ActualSecrets    = 4
)

var _ = Describe("ResourceQuota Controller tests", func() {
	var (
		testNamespace *corev1.Namespace
		resourceQuota *corev1.ResourceQuota
	)

	BeforeEach(func() {
		resourcequota.PerShootQuotaDescriptors = []resourcequota.ResourceQuotaUsages{
			{
				Annotation:       "gardener.cloud/configmaps-per-shoot",
				QuotaKey:         "count/configmaps",
				ExpectedPerShoot: ActualConfigMaps,
			},
			{
				Annotation:       "gardener.cloud/secrets-per-shoot",
				QuotaKey:         "count/secrets",
				ExpectedPerShoot: ActualSecrets,
			},
		}
	})

	Context("ResourceQuota in non-project namespace", func() {
		BeforeEach(func() {
			By("Create test Namespace")
			testNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
			}
			Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
			log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

			DeferCleanup(func() {
				By("Delete test Namespace")
				Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
			})

			By("Create ResourceQuota")
			resourceQuota = &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-resource-quota",
					Namespace: testNamespace.Name,
					Labels:    map[string]string{testID: testRunID},
				},
				Spec: corev1.ResourceQuotaSpec{},
			}
			Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
			log.Info("Created ResourceQuota", "resourceQuota", client.ObjectKeyFromObject(resourceQuota))

			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), &corev1.ResourceQuota{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete ResourceQuota")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
			})
		})

		It("should not have added the gardener created resource annotations to the resource quota", func() {
			Consistently(func(g Gomega) {
				currentResourceQuota := &corev1.ResourceQuota{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), currentResourceQuota)).To(Succeed())
				_, hasAnnotation := currentResourceQuota.Annotations["gardener.cloud/configmaps-per-shoot"]
				g.Expect(hasAnnotation).To(BeFalse())
			}).Should(Succeed())
		})
	})

	Context("ResourceQuota in project namespace", func() {
		var project *gardencorev1beta1.Project

		BeforeEach(func() {
			By("Create test Namespace with project label")
			testNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "garden-",
					Labels: map[string]string{
						testID: testRunID,
					},
				},
			}
			Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
			log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

			DeferCleanup(func() {
				By("Delete test Namespace")
				Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
			})

			By("Create Project")
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-" + utils.ComputeSHA256Hex([]byte(testRunID + CurrentSpecReport().LeafNodeLocation.String()))[:5],
					Labels: map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: &testNamespace.Name,
				},
			}
			Expect(testClient.Create(ctx, project)).To(Succeed())
			log.Info("Created Project", "project", client.ObjectKeyFromObject(project))

			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(project), &gardencorev1beta1.Project{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete Project")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())

				By("Wait for Project to be gone")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
				}).Should(BeNotFoundError())
			})
		})

		Context("Annotations do not exist", func() {
			Context("Quota is enough", func() {
				BeforeEach(func() {
					By("Create ResourceQuota with sufficient quota")
					resourceQuota = &corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-resource-quota",
							Namespace: testNamespace.Name,
							Labels:    map[string]string{testID: testRunID},
						},
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"count/shoots.core.gardener.cloud": resource.MustParse("1"),
								"count/configmaps":                 resource.MustParse("2"),
								"count/secrets":                    resource.MustParse("4"),
							},
						},
					}
					Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
					log.Info("Created ResourceQuota", "resourceQuota", client.ObjectKeyFromObject(resourceQuota))

					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), &corev1.ResourceQuota{})
					}).Should(Succeed())

					DeferCleanup(func() {
						By("Delete ResourceQuota")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
					})
				})

				It("should only add the gardener created resource annotations to the resource quota", func() {
					originalConfigMapQuota := ptr.To(resourceQuota.Spec.Hard["count/configmaps"]).Value()
					originalSecretQuota := ptr.To(resourceQuota.Spec.Hard["count/secrets"]).Value()

					eventuallyExpectActualUsageAnnotations(ctx, resourceQuota)
					consistentlyExpectQuotaSpec(ctx, resourceQuota, originalConfigMapQuota, originalSecretQuota)
				})
			})

			Context("No relevant Quota is specified", func() {
				BeforeEach(func() {
					By("Create ResourceQuota with shoot quota but without configmap/secret quotas")
					resourceQuota = &corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-resource-quota",
							Namespace: testNamespace.Name,
							Labels:    map[string]string{testID: testRunID},
						},
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"count/shoots.core.gardener.cloud": resource.MustParse("1"),
							},
						},
					}
					Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
					log.Info("Created ResourceQuota", "resourceQuota", client.ObjectKeyFromObject(resourceQuota))

					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), &corev1.ResourceQuota{})
					}).Should(Succeed())

					DeferCleanup(func() {
						By("Delete ResourceQuota")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
					})
				})

				It("should only add the gardener created resource annotations to the resource quota", func() {
					eventuallyExpectActualUsageAnnotations(ctx, resourceQuota)
					Consistently(func(g Gomega) {
						currentResourceQuota := &corev1.ResourceQuota{}
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), currentResourceQuota)).To(Succeed())
						g.Expect(currentResourceQuota.Spec.Hard).NotTo(HaveKey("count/configmaps"))
						g.Expect(currentResourceQuota.Spec.Hard).NotTo(HaveKey("count/secrets"))
					}).Should(Succeed())
				})
			})

			Context("Quota is not enough", func() {
				BeforeEach(func() {
					By("Create ResourceQuota with insufficient quota")
					resourceQuota = &corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-resource-quota",
							Namespace: testNamespace.Name,
							Labels:    map[string]string{testID: testRunID},
						},
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"count/shoots.core.gardener.cloud": resource.MustParse("1"),
								"count/configmaps":                 resource.MustParse("1"),
								"count/secrets":                    resource.MustParse("2"),
							},
						},
					}
					Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
					log.Info("Created ResourceQuota", "resourceQuota", client.ObjectKeyFromObject(resourceQuota))

					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), &corev1.ResourceQuota{})
					}).Should(Succeed())

					DeferCleanup(func() {
						By("Delete ResourceQuota")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
					})
				})

				It("should bump the quota to fit the specified number of shoots", func() {
					eventuallyExpectActualUsageAnnotations(ctx, resourceQuota)
					eventuallyExpectQuotaSpec(ctx, resourceQuota, 2, 4)
				})
			})
		})

		Context("Annotations already exist", func() {
			Context("Annotation values are equal to the expected values", func() {
				BeforeEach(func() {
					By("Create ResourceQuota with matching annotations")
					resourceQuota = &corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-resource-quota",
							Namespace: testNamespace.Name,
							Labels:    map[string]string{testID: testRunID},
							Annotations: map[string]string{
								"gardener.cloud/configmaps-per-shoot": "2",
								"gardener.cloud/secrets-per-shoot":    "4",
							},
						},
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"count/shoots.core.gardener.cloud": resource.MustParse("1"),
								"count/configmaps":                 resource.MustParse("2"),
								"count/secrets":                    resource.MustParse("4"),
							},
						},
					}
					Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
					log.Info("Created ResourceQuota", "resourceQuota", client.ObjectKeyFromObject(resourceQuota))

					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), &corev1.ResourceQuota{})
					}).Should(Succeed())

					DeferCleanup(func() {
						By("Delete ResourceQuota")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
					})
				})

				It("should do nothing", func() {
					originalConfigMapQuota := ptr.To(resourceQuota.Spec.Hard["count/configmaps"]).Value()
					originalSecretQuota := ptr.To(resourceQuota.Spec.Hard["count/secrets"]).Value()

					consistentlyExpectUsageAnnotations(ctx, resourceQuota, "2", "4")
					consistentlyExpectQuotaSpec(ctx, resourceQuota, originalConfigMapQuota, originalSecretQuota)
				})
			})

			Context("New annotation value is smaller than the existing one", func() {
				BeforeEach(func() {
					By("Create ResourceQuota with higher annotation values")
					resourceQuota = &corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-resource-quota",
							Namespace: testNamespace.Name,
							Labels:    map[string]string{testID: testRunID},
							Annotations: map[string]string{
								"gardener.cloud/configmaps-per-shoot": "3",
								"gardener.cloud/secrets-per-shoot":    "5",
							},
						},
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"count/shoots.core.gardener.cloud": resource.MustParse("1"),
								"count/configmaps":                 resource.MustParse("3"),
								"count/secrets":                    resource.MustParse("5"),
							},
						},
					}
					Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
					log.Info("Created ResourceQuota", "resourceQuota", client.ObjectKeyFromObject(resourceQuota))

					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), &corev1.ResourceQuota{})
					}).Should(Succeed())

					DeferCleanup(func() {
						By("Delete ResourceQuota")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
					})
				})

				It("should just update the annotations", func() {
					originalConfigMapQuota := ptr.To(resourceQuota.Spec.Hard["count/configmaps"]).Value()
					originalSecretQuota := ptr.To(resourceQuota.Spec.Hard["count/secrets"]).Value()

					consistentlyExpectQuotaSpec(ctx, resourceQuota, originalConfigMapQuota, originalSecretQuota)
					eventuallyExpectActualUsageAnnotations(ctx, resourceQuota)
				})
			})

			Context("New annotation value is bigger than the existing one", func() {
				BeforeEach(func() {
					By("Create ResourceQuota with lower annotation values")
					resourceQuota = &corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-resource-quota",
							Namespace: testNamespace.Name,
							Labels:    map[string]string{testID: testRunID},
							Annotations: map[string]string{
								"gardener.cloud/configmaps-per-shoot": "1",
								"gardener.cloud/secrets-per-shoot":    "3",
							},
						},
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"count/shoots.core.gardener.cloud": resource.MustParse("1"),
								"count/configmaps":                 resource.MustParse("1"),
								"count/secrets":                    resource.MustParse("3"),
							},
						},
					}
					Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
					log.Info("Created ResourceQuota", "resourceQuota", client.ObjectKeyFromObject(resourceQuota))

					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), &corev1.ResourceQuota{})
					}).Should(Succeed())

					DeferCleanup(func() {
						By("Delete ResourceQuota")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
					})
				})

				It("should update the annotations and bump the quota", func() {
					eventuallyExpectQuotaSpec(ctx, resourceQuota, 2, 4)
					eventuallyExpectActualUsageAnnotations(ctx, resourceQuota)
				})
			})
		})
	})
})

func consistentlyExpectUsageAnnotations(ctx context.Context, resourceQuota *corev1.ResourceQuota, configMapsPerShoot, secretsPerShoot string) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), currentResourceQuota)).To(Succeed())
		expectUsageAnnotations(g, currentResourceQuota, configMapsPerShoot, secretsPerShoot)
	}).Should(Succeed())
}

func eventuallyExpectActualUsageAnnotations(ctx context.Context, resourceQuota *corev1.ResourceQuota) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), currentResourceQuota)).To(Succeed())
		expectUsageAnnotations(g, currentResourceQuota, strconv.Itoa(ActualConfigMaps), strconv.Itoa(ActualSecrets))
	}).Should(Succeed())
}

func expectUsageAnnotations(g Gomega, resourceQuota *corev1.ResourceQuota, configMapsPerShoot, secretsPerShoot string) {
	GinkgoHelper()
	configMapAnnotationValue, hasConfigMapAnnotation := resourceQuota.Annotations["gardener.cloud/configmaps-per-shoot"]
	secretAnnotationValue, hasSecretAnnotation := resourceQuota.Annotations["gardener.cloud/secrets-per-shoot"]

	g.Expect(hasConfigMapAnnotation).To(BeTrue(), "expected ConfigMap annotation to be present")
	g.Expect(configMapAnnotationValue).To(Equal(configMapsPerShoot), "expected ConfigMap annotation to have correct value")
	g.Expect(hasSecretAnnotation).To(BeTrue(), "expected secret annotation to be present")
	g.Expect(secretAnnotationValue).To(Equal(secretsPerShoot), "expected secret annotation to have correct value")
}

func eventuallyExpectQuotaSpec(ctx context.Context, resourceQuota *corev1.ResourceQuota, configMapCount, secretCount int64) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), currentResourceQuota)).To(Succeed())
		expectQuotaSpec(g, currentResourceQuota, configMapCount, secretCount)
	}).Should(Succeed())
}

func consistentlyExpectQuotaSpec(ctx context.Context, resourceQuota *corev1.ResourceQuota, configMapCount, secretCount int64) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), currentResourceQuota)).To(Succeed())
		expectQuotaSpec(g, currentResourceQuota, configMapCount, secretCount)
	}).Should(Succeed())
}

func expectQuotaSpec(g Gomega, resourceQuota *corev1.ResourceQuota, configMapCount, secretCount int64) {
	GinkgoHelper()
	g.Expect(resourceQuota.Spec.Hard["count/configmaps"]).To(Equal(resource.MustParse(strconv.FormatInt(configMapCount, 10))))
	g.Expect(resourceQuota.Spec.Hard["count/secrets"]).To(Equal(resource.MustParse(strconv.FormatInt(secretCount, 10))))
}
