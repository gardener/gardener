// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("health", func() {
	var (
		fakeClock = testclock.NewFakeClock(time.Now())
	)

	Describe("ObjectHasAnnotationWithValue", func() {
		var (
			healthFunc health.Func
			key, value string
		)

		BeforeEach(func() {
			key = "foo"
			value = "bar"
			healthFunc = health.ObjectHasAnnotationWithValue(key, value)
		})

		It("should fail if object does not have the annotation", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"other": "bla"},
				},
			})).NotTo(Succeed())
		})
		It("should fail if object's annotation have a different value", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{key: "nope"},
				},
			})).NotTo(Succeed())
		})
		It("should succeed if object's annotation has the expected value", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{key: value},
				},
			})).To(Succeed())
		})
	})

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&health.Clock, fakeClock))
	})

	DescribeTable("#IsSkippedUntil", func(annotations map[string]string, expected bool) {

		skipped := health.IsSkippedUntil(&metav1.ObjectMeta{
			Annotations: annotations,
		})

		Expect(skipped).To(Equal(expected))
	},
		Entry("no annotation: not skipped", nil, false),
		Entry("future timestamp: skipped",
			map[string]string{v1beta1constants.AnnotationCareSkipHealthChecksUntil: fakeClock.Now().Add(1 * time.Hour).Format(time.RFC3339)}, true),
		Entry("past timestamp: not skipped",
			map[string]string{v1beta1constants.AnnotationCareSkipHealthChecksUntil: fakeClock.Now().Add(-1 * time.Hour).Format(time.RFC3339)}, false),
		Entry("invalid RFC3339: not skipped",
			map[string]string{v1beta1constants.AnnotationCareSkipHealthChecksUntil: "not-a-timestamp"}, false),
	)

	Describe("#RemoveExpiredSkipAnnotations", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client
			namespace  = "test-namespace"
			deploy     *appsv1.Deployment
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
					Namespace: namespace,
				},
			}
		})

		It("should do nothing when there are no objects", func() {
			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, namespace,
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			)
		})

		It("should not patch object without annotation", func() {
			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())

			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, namespace,
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
			Expect(deploy.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))
		})

		It("should preserve annotation when timestamp is in the future", func() {
			metav1.SetMetaDataAnnotation(&deploy.ObjectMeta, v1beta1constants.AnnotationCareSkipHealthChecksUntil,
				fakeClock.Now().Add(10*time.Minute).Format(time.RFC3339))
			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())

			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, namespace,
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
			Expect(deploy.Annotations).To(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))
		})

		It("should remove annotation when timestamp has passed", func() {
			metav1.SetMetaDataAnnotation(&deploy.ObjectMeta, v1beta1constants.AnnotationCareSkipHealthChecksUntil,
				fakeClock.Now().Add(-1*time.Minute).Format(time.RFC3339))
			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())

			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, namespace,
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
			Expect(deploy.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))
		})

		It("should preserve annotation when timestamp is not valid RFC3339", func() {
			metav1.SetMetaDataAnnotation(&deploy.ObjectMeta, v1beta1constants.AnnotationCareSkipHealthChecksUntil, "not-a-timestamp")
			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())

			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, namespace,
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
			Expect(deploy.Annotations).To(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))
		})

		It("should only remove expired annotations, leaving future ones intact", func() {
			expiredDeploy := deploy
			metav1.SetMetaDataAnnotation(&expiredDeploy.ObjectMeta, v1beta1constants.AnnotationCareSkipHealthChecksUntil,
				fakeClock.Now().Add(-1*time.Minute).Format(time.RFC3339))
			Expect(fakeClient.Create(ctx, expiredDeploy)).To(Succeed())

			deploy2 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy-2",
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.AnnotationCareSkipHealthChecksUntil: fakeClock.Now().Add(10 * time.Minute).Format(time.RFC3339),
					},
				},
			}
			Expect(fakeClient.Create(ctx, deploy2)).To(Succeed())

			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, namespace,
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expiredDeploy), expiredDeploy)).To(Succeed())
			Expect(expiredDeploy.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy2), deploy2)).To(Succeed())
			Expect(deploy2.Annotations).To(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))
		})

		It("should process multiple GVKs in parallel", func() {
			expiredAnnotation := map[string]string{
				v1beta1constants.AnnotationCareSkipHealthChecksUntil: fakeClock.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			}

			metav1.SetMetaDataAnnotation(&deploy.ObjectMeta, v1beta1constants.AnnotationCareSkipHealthChecksUntil,
				fakeClock.Now().Add(-1*time.Minute).Format(time.RFC3339))
			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())

			prometheus := &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{Name: "test-prometheus", Namespace: namespace, Annotations: expiredAnnotation},
			}
			Expect(fakeClient.Create(ctx, prometheus)).To(Succeed())

			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: "test-etcd", Namespace: namespace, Annotations: expiredAnnotation},
			}
			Expect(fakeClient.Create(ctx, etcd)).To(Succeed())

			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: "test-mr", Namespace: namespace, Annotations: expiredAnnotation},
			}
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, namespace,
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
				druidcorev1alpha1.SchemeGroupVersion.WithKind("EtcdList"),
				monitoringv1.SchemeGroupVersion.WithKind("PrometheusList"),
				resourcesv1alpha1.SchemeGroupVersion.WithKind("ManagedResourceList"),
			)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
			Expect(deploy.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(prometheus), prometheus)).To(Succeed())
			Expect(prometheus.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)).To(Succeed())
			Expect(etcd.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			Expect(mr.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))
		})

		It("should not affect objects in a different namespace", func() {
			metav1.SetMetaDataAnnotation(&deploy.ObjectMeta, v1beta1constants.AnnotationCareSkipHealthChecksUntil,
				fakeClock.Now().Add(-1*time.Minute).Format(time.RFC3339))
			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())

			health.RemoveExpiredSkipAnnotations(ctx, logr.Discard(), fakeClient, "other-namespace",
				appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
			Expect(deploy.Annotations).To(HaveKey(v1beta1constants.AnnotationCareSkipHealthChecksUntil))
		})
	})
})
