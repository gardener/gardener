// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
)

var _ = Describe("HealthStatusChanged", func() {
	var (
		log logr.Logger
		p   predicate.Predicate
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		p = HealthStatusChanged(log)
	})

	Context("metadata-only events", func() {
		var (
			obj *metav1.PartialObjectMetadata
		)

		BeforeEach(func() {
			obj = &metav1.PartialObjectMetadata{}
			obj.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
			obj.SetResourceVersion("1")
		})

		It("should return true for Create", func() {
			Expect(p.Create(event.CreateEvent{Object: obj})).To(BeTrue())
		})

		It("should return true for Delete", func() {
			Expect(p.Delete(event.DeleteEvent{Object: obj})).To(BeTrue())
		})

		It("should return true for cache resyncs", func() {
			objOld := obj.DeepCopy()
			Expect(p.Update(event.UpdateEvent{ObjectOld: objOld, ObjectNew: obj})).To(BeTrue())
		})

		It("should ignore Update", func() {
			objOld := obj.DeepCopy()
			obj.SetResourceVersion("2")
			Expect(p.Update(event.UpdateEvent{ObjectOld: objOld, ObjectNew: obj})).To(BeFalse())
		})

		It("should ignore Generic", func() {
			Expect(p.Generic(event.GenericEvent{Object: obj})).To(BeFalse())
		})
	})

	Context("typed events", func() {
		var (
			healthy, unhealthy *appsv1.Deployment
		)

		BeforeEach(func() {
			healthy = &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				}}},
			}
			healthy.SetResourceVersion("1")
			unhealthy = &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				}}},
			}
			unhealthy.SetResourceVersion("1")
		})

		It("should return true for Create", func() {
			Expect(p.Create(event.CreateEvent{Object: healthy})).To(BeTrue())
			Expect(p.Create(event.CreateEvent{Object: unhealthy})).To(BeTrue())
		})

		It("should return false for Create if object is skipped", func() {
			metav1.SetMetaDataAnnotation(&healthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")
			metav1.SetMetaDataAnnotation(&unhealthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")

			Expect(p.Create(event.CreateEvent{Object: healthy})).To(BeFalse())
			Expect(p.Create(event.CreateEvent{Object: unhealthy})).To(BeFalse())
		})

		It("should return true for Delete", func() {
			Expect(p.Delete(event.DeleteEvent{Object: healthy})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: unhealthy})).To(BeTrue())
		})

		It("should return false for Delete if object is skipped", func() {
			metav1.SetMetaDataAnnotation(&healthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")
			metav1.SetMetaDataAnnotation(&unhealthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")

			Expect(p.Delete(event.DeleteEvent{Object: healthy})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: unhealthy})).To(BeFalse())
		})

		It("should return true for cache resyncs", func() {
			healthyOld := healthy.DeepCopy()
			unhealthyOld := unhealthy.DeepCopy()

			Expect(p.Update(event.UpdateEvent{ObjectOld: healthyOld, ObjectNew: healthy})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectOld: unhealthyOld, ObjectNew: unhealthy})).To(BeTrue())
		})

		It("should return true for Update, if the health status has changed", func() {
			healthyOld := healthy.DeepCopy()
			healthyOld.SetResourceVersion("2")
			unhealthyOld := unhealthy.DeepCopy()
			unhealthyOld.SetResourceVersion("2")

			Expect(p.Update(event.UpdateEvent{ObjectOld: healthyOld, ObjectNew: unhealthy})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectOld: unhealthyOld, ObjectNew: healthy})).To(BeTrue())
		})

		It("should ignore Update, if the health status has not changed", func() {
			healthyOld := healthy.DeepCopy()
			healthyOld.SetResourceVersion("2")
			unhealthyOld := unhealthy.DeepCopy()
			unhealthyOld.SetResourceVersion("2")

			Expect(p.Update(event.UpdateEvent{ObjectOld: healthyOld, ObjectNew: healthy})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectOld: unhealthyOld, ObjectNew: unhealthy})).To(BeFalse())
		})

		It("should ignore Generic", func() {
			Expect(p.Generic(event.GenericEvent{Object: healthy})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: unhealthy})).To(BeFalse())
		})
	})

	Describe("#MapToOriginManagedResource", func() {
		var (
			ctx = context.TODO()
			obj *corev1.Secret
		)

		BeforeEach(func() {
			obj = &corev1.Secret{}
		})

		It("should return nil because the origin annotation is not present", func() {
			Expect(MapToOriginManagedResource(log, "")(ctx, obj)).To(BeNil())
		})

		It("should return nil because the origin annotation cannot be parsed", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/origin": "foo"}

			Expect(MapToOriginManagedResource(log, "")(ctx, obj)).To(BeNil())
		})

		It("should return nil because the origin does not have a cluster id", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/origin": "foo/bar"}

			Expect(MapToOriginManagedResource(log, "foo")(ctx, obj)).To(BeNil())
		})

		It("should return nil because the origin does not match the cluster id", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/origin": "foo:bar/baz"}

			Expect(MapToOriginManagedResource(log, "hugo")(ctx, obj)).To(BeNil())
		})

		It("should return a request because the origin matches the cluster id", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/origin": "foo:bar/baz"}

			Expect(MapToOriginManagedResource(log, "foo")(ctx, obj)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: "baz", Namespace: "bar"}}))
		})
	})
})
