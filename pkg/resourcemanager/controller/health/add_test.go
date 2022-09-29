// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package health_test

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	predicate2 "sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health"
)

var _ = Describe("HealthStatusChanged", func() {
	var (
		log       logr.Logger
		predicate predicate2.Predicate
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		predicate = HealthStatusChanged(log)
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
			Expect(predicate.Create(event.CreateEvent{Object: obj})).To(BeTrue())
		})

		It("should return true for Delete", func() {
			Expect(predicate.Delete(event.DeleteEvent{Object: obj})).To(BeTrue())
		})

		It("should return true for cache resyncs", func() {
			objOld := obj.DeepCopy()
			Expect(predicate.Update(event.UpdateEvent{ObjectOld: objOld, ObjectNew: obj})).To(BeTrue())
		})

		It("should ignore Update", func() {
			objOld := obj.DeepCopy()
			obj.SetResourceVersion("2")
			Expect(predicate.Update(event.UpdateEvent{ObjectOld: objOld, ObjectNew: obj})).To(BeFalse())
		})

		It("should ignore Generic", func() {
			Expect(predicate.Generic(event.GenericEvent{Object: obj})).To(BeFalse())
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
			Expect(predicate.Create(event.CreateEvent{Object: healthy})).To(BeTrue())
			Expect(predicate.Create(event.CreateEvent{Object: unhealthy})).To(BeTrue())
		})

		It("should return false for Create if object is skipped", func() {
			metav1.SetMetaDataAnnotation(&healthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")
			metav1.SetMetaDataAnnotation(&unhealthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")

			Expect(predicate.Create(event.CreateEvent{Object: healthy})).To(BeFalse())
			Expect(predicate.Create(event.CreateEvent{Object: unhealthy})).To(BeFalse())
		})

		It("should return true for Delete", func() {
			Expect(predicate.Delete(event.DeleteEvent{Object: healthy})).To(BeTrue())
			Expect(predicate.Delete(event.DeleteEvent{Object: unhealthy})).To(BeTrue())
		})

		It("should return false for Delete if object is skipped", func() {
			metav1.SetMetaDataAnnotation(&healthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")
			metav1.SetMetaDataAnnotation(&unhealthy.ObjectMeta, "resources.gardener.cloud/skip-health-check", "true")

			Expect(predicate.Delete(event.DeleteEvent{Object: healthy})).To(BeFalse())
			Expect(predicate.Delete(event.DeleteEvent{Object: unhealthy})).To(BeFalse())
		})

		It("should return true for cache resyncs", func() {
			healthyOld := healthy.DeepCopy()
			unhealthyOld := unhealthy.DeepCopy()

			Expect(predicate.Update(event.UpdateEvent{ObjectOld: healthyOld, ObjectNew: healthy})).To(BeTrue())
			Expect(predicate.Update(event.UpdateEvent{ObjectOld: unhealthyOld, ObjectNew: unhealthy})).To(BeTrue())
		})

		It("should return true for Update, if the health status has changed", func() {
			healthyOld := healthy.DeepCopy()
			healthyOld.SetResourceVersion("2")
			unhealthyOld := unhealthy.DeepCopy()
			unhealthyOld.SetResourceVersion("2")

			Expect(predicate.Update(event.UpdateEvent{ObjectOld: healthyOld, ObjectNew: unhealthy})).To(BeTrue())
			Expect(predicate.Update(event.UpdateEvent{ObjectOld: unhealthyOld, ObjectNew: healthy})).To(BeTrue())
		})

		It("should ignore Update, if the health status has not changed", func() {
			healthyOld := healthy.DeepCopy()
			healthyOld.SetResourceVersion("2")
			unhealthyOld := unhealthy.DeepCopy()
			unhealthyOld.SetResourceVersion("2")

			Expect(predicate.Update(event.UpdateEvent{ObjectOld: healthyOld, ObjectNew: healthy})).To(BeFalse())
			Expect(predicate.Update(event.UpdateEvent{ObjectOld: unhealthyOld, ObjectNew: unhealthy})).To(BeFalse())
		})

		It("should ignore Generic", func() {
			Expect(predicate.Generic(event.GenericEvent{Object: healthy})).To(BeFalse())
			Expect(predicate.Generic(event.GenericEvent{Object: unhealthy})).To(BeFalse())
		})
	})

	It("should ignore Update if old GVK cannot be determined", func() {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		objOld := obj.DeepCopy()
		obj.SetResourceVersion("2")
		objOld.SetGroupVersionKind(schema.GroupVersionKind{})
		Expect(predicate.Update(event.UpdateEvent{ObjectOld: objOld, ObjectNew: obj})).To(BeFalse())
	})

	It("should ignore Update if new GVK cannot be determined", func() {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		objOld := obj.DeepCopy()
		obj.SetResourceVersion("2")
		obj.SetGroupVersionKind(schema.GroupVersionKind{})
		Expect(predicate.Update(event.UpdateEvent{ObjectOld: objOld, ObjectNew: obj})).To(BeFalse())
	})
})
