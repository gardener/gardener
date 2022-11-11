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

package networkpolicy_test

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("Add", func() {
	var (
		reconciler *Reconciler
	)

	BeforeEach(func() {
		reconciler = &Reconciler{
			GardenNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1beta1constants.GardenNamespace,
				},
			},
			IstioSystemNamespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1beta1constants.IstioSystemNamespace,
				},
			},
		}
	})

	Describe("#MapToNamespaces", func() {
		var (
			ctrl              *gomock.Controller
			mockRuntimeClient *mockclient.MockClient
			ctx               = context.TODO()
			log               = logr.Discard()
			namespaces        []corev1.Namespace
		)

		BeforeEach(func() {
			namespaces = []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "shoot-foo",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "shoot-bar",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
					},
				},
			}
		})

		JustBeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRuntimeClient = mockclient.NewMockClient(ctrl)
			reconciler.SeedClient = mockRuntimeClient
			// reconciler.ShootNamespaceSelector = shootNamespaceSelector
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return a request with the shoot, gardener and istio-system namespaces' names", func() {
			mockRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{}), &client.ListOptions{LabelSelector: nil}).DoAndReturn(
				func(_ context.Context, shootList *corev1.NamespaceList, _ ...client.ListOption) error {
					shootList.Items = namespaces
					return nil
				},
			)
			Expect(reconciler.MapToNamespaces(ctx, log, nil, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: reconciler.GardenNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: reconciler.IstioSystemNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-foo"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-bar"}},
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
