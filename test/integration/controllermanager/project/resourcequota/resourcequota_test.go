// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Project ResourceQuota controller tests", func() {
	var (
		project       *gardencorev1beta1.Project
		testNamespace *corev1.Namespace
	)

	BeforeEach(func() {
		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
				GenerateName: "garden-" + testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
		}
		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

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

	It("should adapt the ResourceQuota if its limits contradict the specified configmap and secret limits", func() {
		By("Create ResourceQuota that has contradicting limits")
		resourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-resourcequota-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/shoots.core.gardener.cloud": resource.MustParse("2"),
					"count/secrets":                    resource.MustParse("1"),
					"count/configmaps":                 resource.MustParse("1"),
				},
			},
		}

		Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
		})

		assertLimitsUpdated(ctx, resourceQuota, corev1.ResourceList{
			"count/shoots.core.gardener.cloud": resource.MustParse("2"),
			"count/secrets":                    resource.MustParse("8"),
			"count/configmaps":                 resource.MustParse("4"),
		})
	})

	It("should dynamically adapt ResourceQuota limits when no Shoot limit is present and Shoots get created", func() {
		By("Create ResourceQuota without Shoot limit")
		resourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-resourcequota-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/secrets":    resource.MustParse("1"),
					"count/configmaps": resource.MustParse("1"),
				},
			},
		}

		Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
		})

		By("Creating a Shoot in the Project namespace")
		shoot := generateShootManifest(testNamespace.Name)

		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		})

		By("Confirming that ResourceQuota limits have been adapted to fit the new Shoot")
		assertLimitsUpdated(ctx, resourceQuota, corev1.ResourceList{
			"count/secrets":    resource.MustParse("4"),
			"count/configmaps": resource.MustParse("2"),
		})

		By("Creating another Shoot in the Project namespace")
		shoot2 := generateShootManifest(testNamespace.Name)

		Expect(testClient.Create(ctx, shoot2)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot2))).To(Succeed())
		})

		By("Confirming that ResourceQuota limits have been adapted to fit the new Shoot")
		assertLimitsUpdated(ctx, resourceQuota, corev1.ResourceList{
			"count/secrets":    resource.MustParse("8"),
			"count/configmaps": resource.MustParse("4"),
		})
	})

	It("should not modify ResourceQuota if it already complies with configmap and secret limits", func() {
		resourceQuota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-resourcequota-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/shoots.core.gardener.cloud": resource.MustParse("2"),
					"count/secrets":                    resource.MustParse("40"),
					"count/configmaps":                 resource.MustParse("40"),
				},
			},
		}

		Expect(testClient.Create(ctx, resourceQuota)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, resourceQuota))).To(Succeed())
		})

		By("Wait until the ResourceQuota is fetchable by the manager client")
		Eventually(func() error { return mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), resourceQuota) }).Should(Succeed())

		By("Ensure that the ResourceQuota is not modified")
		assertLimitsUnchanged(ctx, resourceQuota)
	})
})

func assertLimitsUpdated(ctx context.Context, resourceQuota *corev1.ResourceQuota, expectedLimits corev1.ResourceList) {
	EventuallyWithOffset(1, func(g Gomega) corev1.ResourceList {
		current := &corev1.ResourceQuota{}
		g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), current)).To(Succeed())
		return current.Spec.Hard
	}).Should(Equal(expectedLimits))
}

func assertLimitsUnchanged(ctx context.Context, resourceQuota *corev1.ResourceQuota) {
	Consistently(func(g Gomega) corev1.ResourceList {
		current := &corev1.ResourceQuota{}
		g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(resourceQuota), current)).To(Succeed())
		return current.Spec.Hard
	}).Should(Equal(resourceQuota.Spec.Hard))
}

func generateShootManifest(namespace string) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-shoot-",
			Namespace:    namespace,
			Labels:       map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.ShootSpec{
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: "1.33.0",
			},
			CloudProfile: &gardencorev1beta1.CloudProfileReference{
				Name: "dummy-cloudprofile",
			},
			Provider: gardencorev1beta1.Provider{
				Type: "dummy-provider",
			},
			Region: "dummy-region",
		},
	}
}
