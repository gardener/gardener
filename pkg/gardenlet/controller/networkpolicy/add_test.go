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

package networkpolicy_test

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("#NetworkPolicyPredicate", func() {
		var (
			p             predicate.Predicate
			networkPolicy *networkingv1.NetworkPolicy
		)

		BeforeEach(func() {
			p = reconciler.NetworkPolicyPredicate()
			networkPolicy = &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"}}
		})

		It("should return true because the NetworkPolicy has name 'allow-to-seed-apiserver'", func() {
			networkPolicy.Name = "allow-to-seed-apiserver"
			Expect(p.Create(event.CreateEvent{Object: networkPolicy})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: networkPolicy})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: networkPolicy})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: networkPolicy})).To(BeTrue())
		})

		It("should return true because the NetworkPolicy has name 'allow-to-runtime-apiserver'", func() {
			networkPolicy.Name = "allow-to-runtime-apiserver"
			Expect(p.Create(event.CreateEvent{Object: networkPolicy})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: networkPolicy})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: networkPolicy})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: networkPolicy})).To(BeTrue())
		})

		It("should return false because the NetworkPolicy is not managed by this reconciler", func() {
			networkPolicy.Name = "not-managed"
			Expect(p.Create(event.CreateEvent{Object: networkPolicy})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: networkPolicy})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: networkPolicy})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: networkPolicy})).To(BeFalse())
		})
	})

	Describe("#ClusterPredicate", func() {
		var (
			p       predicate.Predicate
			cluster *extensionsv1alpha1.Cluster
			shoot   *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			p = reconciler.ClusterPredicate()
			shoot = &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "garden-bar",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{},
					},
				},
			}
			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Raw: encode(shoot),
					},
				},
			}
		})

		It("should return false if networking is nil for workerless shoot", func() {
			newCluster := cluster.DeepCopy()

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if networking is updated to non-nil for workerless shoot", func() {
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if service cidr is changed for workerless shoot", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: pointer.String("foo")}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: pointer.String("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return false if no change in networking for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Pods: pointer.String("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if pods cidr is changed for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Pods: pointer.String("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Pods: pointer.String("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if services cidr is changed for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: pointer.String("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: pointer.String("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if nodes cidr is changed for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Nodes: pointer.String("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Nodes: pointer.String("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})
	})

	Describe("#MapToNamespaces", func() {
		var (
			ctx        = context.TODO()
			log        = logr.Discard()
			fakeClient client.Client

			gardenNamespace             *corev1.Namespace
			istioSystemNamespace        *corev1.Namespace
			istioIngressNamespace       *corev1.Namespace
			istioExposureClassNamespace *corev1.Namespace
			shootNamespace              *corev1.Namespace
			extensionNamespace          *corev1.Namespace
			fooNamespace                *corev1.Namespace
		)

		BeforeEach(func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.FullNetworkPoliciesInRuntimeCluster, true))

			fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			reconciler.RuntimeClient = fakeClient

			gardenNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "garden-1238as",
					Labels: map[string]string{v1beta1constants.LabelRole: v1beta1constants.GardenNamespace},
				},
			}
			istioSystemNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "istio-system-123a4",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem},
				},
			}
			istioIngressNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "istio-ingress-123a4",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress},
				},
			}
			istioExposureClassNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "istio-ingress-handler-foo-123a4",
					Labels: map[string]string{v1beta1constants.LabelExposureClassHandlerName: ""},
				},
			}
			shootNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "shoot--bar",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
				},
			}
			extensionNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "extension-baz",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension},
				},
			}
			fooNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		})

		It("should return a request with the relevant namespaces' names", func() {
			Expect(fakeClient.Create(ctx, gardenNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, istioSystemNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, istioIngressNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, istioExposureClassNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, extensionNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, fooNamespace)).To(Succeed())

			Expect(reconciler.MapToNamespaces(ctx, log, nil, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: gardenNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: istioSystemNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: istioIngressNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: istioExposureClassNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: shootNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: extensionNamespace.Name}},
			))
		})
	})

	Describe("#MapObjectToNamespace", func() {
		var (
			ctx           = context.TODO()
			log           = logr.Discard()
			networkpolicy *networkingv1.NetworkPolicy
		)

		BeforeEach(func() {
			networkpolicy = &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
		})

		It("should return a request with the namespace's name", func() {
			Expect(reconciler.MapObjectToNamespace(ctx, log, nil, networkpolicy)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: networkpolicy.Namespace}},
			))
		})
	})

	Describe("#IsKubernetesEndpoint", func() {
		var (
			p        predicate.Predicate
			endpoint *corev1.Endpoints
		)

		BeforeEach(func() {
			p = reconciler.IsKubernetesEndpoint()
			endpoint = &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"}}
		})

		It("should return true because the endpoint is the Kubernetes endpoint", func() {
			Expect(p.Create(event.CreateEvent{Object: endpoint})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: endpoint})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: endpoint})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: endpoint})).To(BeTrue())
		})

		It("should return false because the endpoint is not the Kubernetes endpoint", func() {
			endpoint.Name = "foo"

			Expect(p.Create(event.CreateEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: endpoint})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: endpoint})).To(BeFalse())
		})

		It("should return false because the endpoint is a Kubernetes endpoint in a different namespace", func() {
			endpoint.Namespace = "bar"

			Expect(p.Create(event.CreateEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: endpoint})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: endpoint})).To(BeFalse())
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
