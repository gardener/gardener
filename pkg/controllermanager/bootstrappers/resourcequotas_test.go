// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("#bumpProjectResourceQuotas", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
		logger       = GinkgoLogr
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    logger,
			Client: fakeClient,
		}
	})

	It("should handle no project namespaces gracefully", func() {
		Expect(bootstrapper.bumpProjectResourceQuotas(ctx)).To(Succeed())
	})

	It("should handle project namespace with no resource quotas", func() {
		// Create a project namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectNS,
				Labels: map[string]string{
					"gardener.cloud/role": "project",
				},
			},
		}
		Expect(fakeClient.Create(ctx, ns)).To(Succeed())

		Expect(bootstrapper.bumpProjectResourceQuotas(ctx)).To(Succeed())
	})
})

var _ = Describe("#alignResourceQuota", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
		logger       = GinkgoLogr
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    logger,
			Client: fakeClient,
		}
	})

	It("should skip resource quota with deletion timestamp", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
				DeletionTimestamp: &metav1.Time{
					Time: metav1.Now().Add(-1),
				},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/configmaps": *resource.NewQuantity(10, resource.DecimalSI),
				},
			},
		}

		Expect(bootstrapper.alignResourceQuota(ctx, logger, *rq, projectNS)).To(Succeed())
	})
})

var _ = Describe("#handleMissingAnnotation", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
		logger       = GinkgoLogr
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    logger,
			Client: fakeClient,
		}
	})

	It("should set annotation when no quota exists and no shoots are present", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		err := bootstrapper.handleMissingAnnotation(ctx, logger, rq, configMapsPerShootAnnotation, "count/configmaps", 5, projectNS)
		Expect(err).To(Succeed())

		// Verify the resource quota was updated
		updatedRQ := &corev1.ResourceQuota{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rq), updatedRQ)).To(Succeed())
		Expect(updatedRQ.Annotations).To(HaveKeyWithValue(configMapsPerShootAnnotation, "5"))
	})

	It("should bump quota when current is less than required", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/configmaps": *resource.NewQuantity(5, resource.DecimalSI),
				},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		// Create a project namespace with 2 shoots
		projectNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectNS,
			},
		}
		Expect(fakeClient.Create(ctx, projectNamespace)).To(Succeed())

		// Create 2 shoots
		for i := 0; i < 2; i++ {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-" + string(rune('0'+i)),
					Namespace: projectNS,
				},
			}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
		}

		err := bootstrapper.handleMissingAnnotation(ctx, logger, rq, configMapsPerShootAnnotation, "count/configmaps", 5, projectNS)
		Expect(err).To(Succeed())

		// Verify the quota was bumped
		updatedRQ := &corev1.ResourceQuota{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rq), updatedRQ)).To(Succeed())
		Expect(ptr.To(updatedRQ.Spec.Hard["count/configmaps"]).Value()).To(Equal(int64(10))) // 5 * 2 shoots
		Expect(updatedRQ.Annotations).To(HaveKeyWithValue(configMapsPerShootAnnotation, "5"))
	})
})

var _ = Describe("#handleAnnotationMismatch", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
		logger       = GinkgoLogr
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    logger,
			Client: fakeClient,
		}

		// Create a project namespace with 3 shoots
		projectNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectNS,
			},
		}
		Expect(fakeClient.Create(ctx, projectNamespace)).To(Succeed())

		for i := 0; i < 3; i++ {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-" + string(rune('0'+i)),
					Namespace: projectNS,
				},
			}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
		}
	})

	It("should increase quota when annotation count is less than expected", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
				Annotations: map[string]string{
					configMapsPerShootAnnotation: "3",
				},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/configmaps": *resource.NewQuantity(9, resource.DecimalSI), // 3 * 3 shoots
				},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		err := bootstrapper.handleAnnotationMismatch(ctx, logger, rq, configMapsPerShootAnnotation, "count/configmaps", 3, 5, projectNS)
		Expect(err).To(Succeed())

		// Verify the quota was updated: old 9 + (5-3)*3 = 9 + 6 = 15
		updatedRQ := &corev1.ResourceQuota{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rq), updatedRQ)).To(Succeed())
		Expect(ptr.To(updatedRQ.Spec.Hard["count/configmaps"]).Value()).To(Equal(int64(15)))
	})

	It("should not decrease quota when annotation count is greater than expected", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
				Annotations: map[string]string{
					configMapsPerShootAnnotation: "10",
				},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/configmaps": *resource.NewQuantity(30, resource.DecimalSI),
				},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		err := bootstrapper.handleAnnotationMismatch(ctx, logger, rq, configMapsPerShootAnnotation, "count/configmaps", 10, 5, projectNS)
		Expect(err).To(Succeed())

		// Verify the quota was not changed (annotation count is greater)
		updatedRQ := &corev1.ResourceQuota{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rq), updatedRQ)).To(Succeed())
		Expect(ptr.To(updatedRQ.Spec.Hard["count/configmaps"]).Value()).To(Equal(int64(30)))
	})
})

var _ = Describe("#updateResourceQuotaHard", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    GinkgoLogr,
			Client: fakeClient,
		}
	})

	It("should update resource quota hard limit", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/configmaps": *resource.NewQuantity(10, resource.DecimalSI),
				},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		err := bootstrapper.updateResourceQuotaHard(ctx, rq, "count/configmaps", "20")
		Expect(err).To(Succeed())

		// Verify the resource quota was updated
		updatedRQ := &corev1.ResourceQuota{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rq), updatedRQ)).To(Succeed())
		Expect(ptr.To(updatedRQ.Spec.Hard["count/configmaps"]).Value()).To(Equal(int64(20)))
	})
})

var _ = Describe("#setResourceQuotaAnnotation", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
		logger       = GinkgoLogr
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    logger,
			Client: fakeClient,
		}
	})

	It("should set annotation on resource quota", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		err := bootstrapper.setResourceQuotaAnnotation(ctx, rq, configMapsPerShootAnnotation, "5")
		Expect(err).To(Succeed())

		// Verify the annotation was set
		updatedRQ := &corev1.ResourceQuota{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rq), updatedRQ)).To(Succeed())
		Expect(updatedRQ.Annotations).To(HaveKeyWithValue(configMapsPerShootAnnotation, "5"))
	})

	It("should update existing annotation on resource quota", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
				Annotations: map[string]string{
					configMapsPerShootAnnotation: "3",
				},
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		err := bootstrapper.setResourceQuotaAnnotation(ctx, rq, configMapsPerShootAnnotation, "5")
		Expect(err).To(Succeed())

		// Verify the annotation was updated
		updatedRQ := &corev1.ResourceQuota{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rq), updatedRQ)).To(Succeed())
		Expect(updatedRQ.Annotations).To(HaveKeyWithValue(configMapsPerShootAnnotation, "5"))
	})
})

var _ = Describe("#getResourceQuotas", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    GinkgoLogr,
			Client: fakeClient,
		}
	})

	It("should return empty list when no resource quotas exist", func() {
		quotas, err := bootstrapper.getResourceQuotas(ctx, projectNS)
		Expect(err).To(Succeed())
		Expect(quotas.Items).To(BeEmpty())
	})

	It("should return resource quotas in namespace", func() {
		// Create namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectNS,
			},
		}
		Expect(fakeClient.Create(ctx, ns)).To(Succeed())

		// Create resource quota
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/configmaps": *resource.NewQuantity(10, resource.DecimalSI),
				},
			},
		}
		Expect(fakeClient.Create(ctx, rq)).To(Succeed())

		quotas, err := bootstrapper.getResourceQuotas(ctx, projectNS)
		Expect(err).To(Succeed())
		Expect(quotas.Items).To(HaveLen(1))
		Expect(quotas.Items[0].Name).To(Equal("test-quota"))
	})
})

var _ = Describe("#getMaximumShootsInProject", func() {
	var (
		bootstrapper *Bootstrapper
		fakeClient   client.Client
		ctx          = context.TODO()
		projectNS    = "garden-project-test"
		logger       = GinkgoLogr
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		bootstrapper = &Bootstrapper{
			Log:    logger,
			Client: fakeClient,
		}

		// Create project namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectNS,
			},
		}
		Expect(fakeClient.Create(ctx, ns)).To(Succeed())
	})

	It("should use quota when shoot count resource is specified", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceName("count/shoots.core.gardener.cloud"): resource.MustParse("5"),
				},
			},
		}

		maximum, err := bootstrapper.getMaximumShootsInProject(ctx, *rq, projectNS)
		Expect(err).To(Succeed())
		Expect(maximum).To(Equal(int64(5)))
	})

	It("should count shoots when quota is not specified", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{},
			},
		}

		// Create 3 shoots
		for i := 0; i < 3; i++ {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("shoot-%d", i),
					Namespace: projectNS,
				},
			}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
		}

		maximum, err := bootstrapper.getMaximumShootsInProject(ctx, *rq, projectNS)
		Expect(err).To(Succeed())
		Expect(maximum).To(Equal(int64(3)))
	})

	It("should return 0 when no shoots exist and no quota is specified", func() {
		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-quota",
				Namespace: projectNS,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{},
			},
		}

		maximum, err := bootstrapper.getMaximumShootsInProject(ctx, *rq, projectNS)
		Expect(err).To(Succeed())
		Expect(maximum).To(Equal(int64(0)))
	})
})
