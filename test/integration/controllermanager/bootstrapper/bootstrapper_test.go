// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrapper_test

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
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/bootstrappers"
)

var _ = Describe("Bootstrapper Controller", func() {
	var (
		bootstrapper manager.Runnable
		testCtx      context.Context
	)

	BeforeEach(func() {
		bootstrapper = &bootstrappers.Bootstrapper{
			Log:        GinkgoLogr,
			Client:     mgr.GetClient(),
			RESTConfig: mgr.GetConfig(),
		}
		testCtx = context.Background()

		bootstrappers.PerShootQuotaDescriptors = []bootstrappers.ResourceQuotaUsages{
			{
				Annotation:       bootstrappers.ConfigMapsPerShootAnnotation,
				QuotaKey:         "count/configmaps",
				ExpectedPerShoot: 2,
			},
			{
				Annotation:       bootstrappers.SecretsPerShootAnnotation,
				QuotaKey:         "count/secrets",
				ExpectedPerShoot: 4,
			},
		}
	})

	Context("ResourceQuota in non project namespace", func() {
		var (
			namespace     *corev1.Namespace
			resourceQuota *corev1.ResourceQuota
		)

		BeforeEach(func() {
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID,
				},
			}
			Expect(testClient.Create(ctx, namespace)).To(Succeed())

			resourceQuota = &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-resource-quota",
					Namespace: namespace.Name,
				},
				Spec: corev1.ResourceQuotaSpec{},
			}
			Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, resourceQuota)).To(Succeed())
			Expect(testClient.Delete(ctx, namespace)).To(Succeed())
		})

		It("should not have added the gardener created resource annotations to the resource quota", func() {
			Expect(bootstrapper.Start(testCtx)).To(Succeed())

			Consistently(func() bool {
				currentResourceQuota := &corev1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceQuota.Name,
						Namespace: namespace.Name,
					},
				}
				Expect(testClient.Get(testCtx, client.ObjectKey{
					Name:      currentResourceQuota.Name,
					Namespace: namespace.Name,
				}, currentResourceQuota)).To(Succeed())

				_, hasAnnotation := currentResourceQuota.Annotations[bootstrappers.ConfigMapsPerShootAnnotation]
				return hasAnnotation
			}).Should(BeFalse())
		})
	})

	Context("ResourceQuota in project namespace", func() {
		var (
			namespace     *corev1.Namespace
			resourceQuota *corev1.ResourceQuota
		)

		BeforeEach(func() {
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID,
					Labels: map[string]string{
						"gardener.cloud/role": "project",
					},
				},
			}
			Expect(testClient.Create(ctx, namespace)).To(Succeed())

			resourceQuota = &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-resource-quota",
					Namespace: namespace.Name,
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
		})

		Context("Annotations do not exist", func() {
			Context("Quota is enough", func() {
				It("should only add the gardener created resource annotations to the resource quota", func() {

					originalConfigMapQuota := ptr.To(resourceQuota.Spec.Hard["count/configmaps"]).Value()
					originalSecretQuota := ptr.To(resourceQuota.Spec.Hard["count/secrets"]).Value()

					Expect(bootstrapper.Start(testCtx)).To(Succeed())

					eventuallyExpectUsageAnnotations(testCtx, resourceQuota, "2", "4")
					consistentlyExpectQuotaSpec(testCtx, resourceQuota, originalConfigMapQuota, originalSecretQuota)
				})
			})

			Context("No relevant Quota is specified", func() {
				BeforeEach(func() {
					resourceQuota.Spec.Hard = corev1.ResourceList{}
					Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())
				})

				It("should only add the gardener created resource annotations to the resource quota", func() {
					Expect(bootstrapper.Start(testCtx)).To(Succeed())

					eventuallyExpectUsageAnnotations(testCtx, resourceQuota, "2", "4")
					Consistently(func(g Gomega) corev1.ResourceList {
						currentResourceQuota := &corev1.ResourceQuota{}
						g.Expect(testClient.Get(testCtx, client.ObjectKey{
							Name:      resourceQuota.Name,
							Namespace: resourceQuota.Namespace,
						}, currentResourceQuota)).To(Succeed())
						return currentResourceQuota.Spec.Hard
					}).Should(BeEquivalentTo(resourceQuota.Spec.Hard))
				})
			})

			Context("Quota is not enough", func() {
				BeforeEach(func() {
					resourceQuota.Spec.Hard = corev1.ResourceList{
						"count/shoots.core.gardener.cloud": resource.MustParse("1"),
						"count/configmaps":                 resource.MustParse("1"),
						"count/secrets":                    resource.MustParse("2"),
					}
					Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())
				})

				It("should bump the quota to fit the specified number of shoots", func() {
					Expect(bootstrapper.Start(testCtx)).To(Succeed())

					eventuallyExpectUsageAnnotations(testCtx, resourceQuota, "2", "4")
					eventuallyExpectQuotaSpec(testCtx, resourceQuota, 2, 4)
				})
			})
		})

		Context("Annotations already exist", func() {
			BeforeEach(func() {
				resourceQuota.Annotations = map[string]string{
					"gardener.cloud/configmaps-per-shoot": "2",
					"gardener.cloud/secrets-per-shoot":    "4",
				}
				Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())
			})

			Context("Annotation values are equal to the expected values", func() {
				BeforeEach(func() {
					resourceQuota.Spec.Hard = corev1.ResourceList{
						"count/shoots.core.gardener.cloud": resource.MustParse("1"),
						"count/configmaps":                 resource.MustParse("2"),
						"count/secrets":                    resource.MustParse("4"),
					}
					Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())
				})

				It("should do nothing", func() {
					originalConfigMapQuota := ptr.To(resourceQuota.Spec.Hard["count/configmaps"]).Value()
					originalSecretQuota := ptr.To(resourceQuota.Spec.Hard["count/secrets"]).Value()

					Expect(bootstrapper.Start(testCtx)).To(Succeed())

					consistentlyExpectUsageAnnotations(testCtx, resourceQuota, "2", "4")
					consistentlyExpectQuotaSpec(testCtx, resourceQuota, originalConfigMapQuota, originalSecretQuota)
				})
			})

			Context("New annotation value is smaller than the existing one", func() {
				BeforeEach(func() {
					resourceQuota.Spec.Hard = corev1.ResourceList{
						"count/shoots.core.gardener.cloud": resource.MustParse("1"),
						"count/configmaps":                 resource.MustParse("2"),
						"count/secrets":                    resource.MustParse("4"),
					}
					Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())
				})

				It("should just update the annotations", func() {
					originalConfigMapQuota := ptr.To(resourceQuota.Spec.Hard["count/configmaps"]).Value()
					originalSecretQuota := ptr.To(resourceQuota.Spec.Hard["count/secrets"]).Value()

					// Updating the annotations to bigger values than the real ones,
					// a new run of the bootstrapper would simulate that the desired values were changed to smaller ones.
					resourceQuota.Annotations[bootstrappers.ConfigMapsPerShootAnnotation] = "3"
					resourceQuota.Annotations[bootstrappers.SecretsPerShootAnnotation] = "5"
					Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())

					Expect(bootstrapper.Start(testCtx)).To(Succeed())

					consistentlyExpectQuotaSpec(testCtx, resourceQuota, originalConfigMapQuota, originalSecretQuota)
					eventuallyExpectUsageAnnotations(testCtx, resourceQuota, "3", "5")
				})
			})

			Context("New annotation value is bigger than the existing one", func() {
				BeforeEach(func() {
					resourceQuota.Spec.Hard = corev1.ResourceList{
						"count/shoots.core.gardener.cloud": resource.MustParse("1"),
						"count/configmaps":                 resource.MustParse("2"),
						"count/secrets":                    resource.MustParse("4"),
					}
					Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())
				})

				It("should update the annotations and bump the quota", func() {

					// Updating the annotations to smaller values than the real ones,
					// a new run of the bootstrapper would simulate that the desired values were changed to bigger ones.
					resourceQuota.Annotations[bootstrappers.ConfigMapsPerShootAnnotation] = "1"
					resourceQuota.Annotations[bootstrappers.SecretsPerShootAnnotation] = "3"
					Expect(testClient.Update(ctx, resourceQuota)).To(Succeed())

					Expect(bootstrapper.Start(testCtx)).To(Succeed())

					eventuallyExpectQuotaSpec(testCtx, resourceQuota, 3, 5)
					eventuallyExpectUsageAnnotations(testCtx, resourceQuota, "2", "4")
				})
			})
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, resourceQuota)).To(Succeed())
			Expect(testClient.Delete(ctx, namespace)).To(Succeed())
		})
	})
})

func consistentlyExpectUsageAnnotations(ctx context.Context, resourceQuota *corev1.ResourceQuota, configMapsPerShoot, secretsPerShoot string) {
	ConsistentlyWithOffset(1, func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceQuota.Name,
				Namespace: resourceQuota.Namespace,
			},
		}
		Expect(testClient.Get(ctx, client.ObjectKey{
			Name:      currentResourceQuota.Name,
			Namespace: resourceQuota.Namespace,
		}, currentResourceQuota)).To(Succeed())

		expectUsageAnnotations(g, currentResourceQuota, configMapsPerShoot, secretsPerShoot)
	}).To(Succeed())
}

func eventuallyExpectUsageAnnotations(ctx context.Context, resourceQuota *corev1.ResourceQuota, configMapsPerShoot, secretsPerShoot string) {
	EventuallyWithOffset(1, func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceQuota.Name,
				Namespace: resourceQuota.Namespace,
			},
		}
		Expect(testClient.Get(ctx, client.ObjectKey{
			Name:      currentResourceQuota.Name,
			Namespace: resourceQuota.Namespace,
		}, currentResourceQuota)).To(Succeed())

		expectUsageAnnotations(g, currentResourceQuota, configMapsPerShoot, secretsPerShoot)
	}).To(Succeed())
}

func expectUsageAnnotations(g Gomega, resourceQuota *corev1.ResourceQuota, configMapsPerShoot, secretsPerShoot string) {
	configMapAnnotationValue, hasConfigMapAnnotation := resourceQuota.Annotations[bootstrappers.ConfigMapsPerShootAnnotation]
	secretAnnotationValue, hasSecretAnnotation := resourceQuota.Annotations[bootstrappers.SecretsPerShootAnnotation]

	g.ExpectWithOffset(1, hasConfigMapAnnotation).To(BeTrue(), "expected ConfigMap annotation to be present")
	g.ExpectWithOffset(1, configMapAnnotationValue).To(Equal(configMapsPerShoot), "expected ConfigMap annotation to have correct value")
	g.ExpectWithOffset(1, hasSecretAnnotation).To(BeTrue(), "expected secret annotation to be present")
	g.ExpectWithOffset(1, secretAnnotationValue).To(Equal(secretsPerShoot), "expected secret annotation to have correct value")
}

func eventuallyExpectQuotaSpec(ctx context.Context, resourceQuota *corev1.ResourceQuota, configMapCount, secretCount int64) {
	EventuallyWithOffset(1, func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceQuota.Name,
				Namespace: resourceQuota.Namespace,
			},
		}
		Expect(testClient.Get(ctx, client.ObjectKey{
			Name:      currentResourceQuota.Name,
			Namespace: resourceQuota.Namespace,
		}, currentResourceQuota)).To(Succeed())

		expectQuotaSpec(g, currentResourceQuota, configMapCount, secretCount)
	}).To(Succeed())
}

func consistentlyExpectQuotaSpec(ctx context.Context, resourceQuota *corev1.ResourceQuota, configMapCount, secretCount int64) {
	ConsistentlyWithOffset(1, func(g Gomega) {
		currentResourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceQuota.Name,
				Namespace: resourceQuota.Namespace,
			},
		}
		Expect(testClient.Get(ctx, client.ObjectKey{
			Name:      currentResourceQuota.Name,
			Namespace: resourceQuota.Namespace,
		}, currentResourceQuota)).To(Succeed())

		expectQuotaSpec(g, currentResourceQuota, configMapCount, secretCount)
	}).To(Succeed())
}

func expectQuotaSpec(g Gomega, resourceQuota *corev1.ResourceQuota, configMapCount, secretCount int64) {
	g.ExpectWithOffset(1, resourceQuota.Spec.Hard["count/configmaps"]).To(Equal(resource.MustParse(strconv.FormatInt(configMapCount, 10))))
	g.ExpectWithOffset(1, resourceQuota.Spec.Hard["count/secrets"]).To(Equal(resource.MustParse(strconv.FormatInt(secretCount, 10))))
}
