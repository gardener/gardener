// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package progressing_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health/progressing"
)

var _ = Describe("Add", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler *Reconciler
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		reconciler = &Reconciler{TargetClient: fakeClient}
	})

	Describe("#ProgressingStatusChanged", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ProgressingStatusChanged(ctx)
		})

		Describe("#Create", func() {
			It("should return false", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			tests := func(newObjFunc func() client.Object, makeProgressing, makeNotProgressing func(client.Object)) {
				var newObj, oldObj client.Object

				BeforeEach(func() {
					newObj = newObjFunc()
					oldObj = newObjFunc()
				})

				It("should return true for periodic cache resyncs", func() {
					Expect(p.Update(event.UpdateEvent{ObjectNew: newObj, ObjectOld: newObj})).To(BeTrue())
				})

				It("should return false because both old and new not progressing", func() {
					oldObj.SetResourceVersion("2")
					makeNotProgressing(newObj)
					makeNotProgressing(oldObj)
					Expect(p.Update(event.UpdateEvent{ObjectNew: newObj, ObjectOld: oldObj})).To(BeFalse())
				})

				It("should return false because both old and new are progressing", func() {
					oldObj.SetResourceVersion("2")
					makeProgressing(newObj)
					makeProgressing(oldObj)
					Expect(p.Update(event.UpdateEvent{ObjectNew: newObj, ObjectOld: oldObj})).To(BeFalse())
				})

				It("should return true because old is progressing", func() {
					oldObj.SetResourceVersion("2")
					makeNotProgressing(newObj)
					makeProgressing(oldObj)
					Expect(p.Update(event.UpdateEvent{ObjectNew: newObj, ObjectOld: oldObj})).To(BeTrue())
				})

				It("should return true because new is progressing", func() {
					makeProgressing(newObj)
					makeNotProgressing(oldObj)
					Expect(p.Update(event.UpdateEvent{ObjectNew: newObj, ObjectOld: oldObj})).To(BeTrue())
				})

				It("should return false because old progressing but health check skipped", func() {
					oldObj.SetAnnotations(map[string]string{"resources.gardener.cloud/skip-health-check": "true"})
					oldObj.SetResourceVersion("2")
					makeProgressing(oldObj)
					makeNotProgressing(newObj)
					Expect(p.Update(event.UpdateEvent{ObjectNew: newObj, ObjectOld: oldObj})).To(BeFalse())
				})

				It("should return false because new progressing but health check skipped", func() {
					newObj.SetAnnotations(map[string]string{"resources.gardener.cloud/skip-health-check": "true"})
					oldObj.SetResourceVersion("2")
					makeNotProgressing(oldObj)
					makeProgressing(newObj)
					Expect(p.Update(event.UpdateEvent{ObjectNew: newObj, ObjectOld: oldObj})).To(BeFalse())
				})
			}

			Context("deployments", func() {
				tests(
					func() client.Object {
						return &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}}}
					},
					func(obj client.Object) {
						deploy := obj.(*appsv1.Deployment)
						deploy.Generation++
					},
					func(obj client.Object) {
						deploy := obj.(*appsv1.Deployment)
						deploy.Status.ObservedGeneration = deploy.Generation
						deploy.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"}}
					},
				)
			})

			Context("statefulsets", func() {
				tests(
					func() client.Object {
						return &appsv1.StatefulSet{}
					},
					func(obj client.Object) {
						sts := obj.(*appsv1.StatefulSet)
						sts.Generation++
					},
					func(obj client.Object) {
						sts := obj.(*appsv1.StatefulSet)
						sts.Status.ObservedGeneration = sts.Generation
						sts.Status.UpdatedReplicas = 1
					},
				)
			})

			Context("daemonsets", func() {
				tests(
					func() client.Object {
						return &appsv1.DaemonSet{}
					},
					func(obj client.Object) {
						ds := obj.(*appsv1.DaemonSet)
						ds.Generation++
					},
					func(obj client.Object) {
						ds := obj.(*appsv1.DaemonSet)
						ds.Status.ObservedGeneration = ds.Generation
					},
				)
			})

			Context("other resources", func() {
				var secret *corev1.Secret

				BeforeEach(func() {
					secret = &corev1.Secret{}
				})

				It("should return true for periodic cache resyncs", func() {
					Expect(p.Update(event.UpdateEvent{ObjectNew: secret, ObjectOld: secret})).To(BeTrue())
				})

				It("should return false", func() {
					oldSecret := secret.DeepCopy()
					oldSecret.ResourceVersion = "4"
					Expect(p.Update(event.UpdateEvent{ObjectNew: secret, ObjectOld: oldSecret})).To(BeFalse())
				})
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
