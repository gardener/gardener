// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package progressing_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

						pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{GenerateName: "pod-", Labels: map[string]string{"foo": "bar"}}}
						Expect(fakeClient.Create(ctx, pod)).To(Succeed())
						DeferCleanup(func() {
							Expect(fakeClient.Delete(ctx, pod)).To(Succeed())
						})
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

	Describe("#MapPodToDeploymentToOriginManagedResource", func() {
		var (
			log = logr.Discard()

			pod        *corev1.Pod
			replicaSet *appsv1.ReplicaSet
			deployment *appsv1.Deployment
		)

		BeforeEach(func() {
			pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace"}}
			replicaSet = &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "replicaset", Namespace: pod.Namespace}}
			deployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deployment", Namespace: pod.Namespace}}
		})

		It("should return nil because owning Deployment was not found", func() {
			Expect(reconciler.MapPodToDeploymentToOriginManagedResource(log, "")(ctx, pod)).To(BeEmpty())
		})

		It("should return nil because Deployment has no matching origin", func() {
			pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: replicaSet.Name}}
			replicaSet.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name}}
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			Expect(reconciler.MapPodToDeploymentToOriginManagedResource(log, "")(ctx, pod)).To(BeEmpty())
		})

		It("should return a request because the origin of the Deployment matches the cluster id", func() {
			pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: replicaSet.Name}}
			replicaSet.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name}}
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())
			deployment.Annotations = map[string]string{"resources.gardener.cloud/origin": "foo:bar/baz"}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			Expect(reconciler.MapPodToDeploymentToOriginManagedResource(log, "foo")(ctx, pod)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: "baz", Namespace: "bar"}}))
		})
	})
})
