// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionclusterrole_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/extensionclusterrole"
)

var _ = Describe("Add", func() {
	var (
		reconciler     *Reconciler
		serviceAccount *corev1.ServiceAccount
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "seed-foo",
				Name:      "extension-bar",
			},
		}
	})

	Describe("ServiceAccountPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ServiceAccountPredicate()
		})

		tests := func(f func(obj client.Object) bool) {
			It("should return false because object is no Secret", func() {
				Expect(f(&corev1.ConfigMap{})).To(BeFalse())
			})

			It("should return false because namespace is not prefixed with 'seed-'", func() {
				serviceAccount.Namespace = "foo"
				Expect(f(serviceAccount)).To(BeFalse())
			})

			It("should return false because name is not prefixed with 'extension-'", func() {
				serviceAccount.Name = "bar"
				Expect(f(serviceAccount)).To(BeFalse())
			})

			It("should return true because object matches all conditions", func() {
				Expect(f(serviceAccount)).To(BeTrue())
			})
		}

		Describe("#Create", func() {
			tests(func(obj client.Object) bool { return p.Create(event.CreateEvent{Object: obj}) })
		})

		Describe("#Update", func() {
			tests(func(obj client.Object) bool { return p.Update(event.UpdateEvent{ObjectNew: obj}) })
		})

		Describe("#Delete", func() {
			tests(func(obj client.Object) bool { return p.Delete(event.DeleteEvent{Object: obj}) })
		})

		Describe("#Generic", func() {
			tests(func(obj client.Object) bool { return p.Generic(event.GenericEvent{Object: obj}) })
		})
	})

	Describe("#MapToAllClusterRoles", func() {
		var (
			ctx                                      = context.Background()
			log                                      logr.Logger
			fakeClient                               client.Client
			clusterRole1, clusterRole2, clusterRole3 *rbacv1.ClusterRole
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

			clusterRole1 = &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "clusterRole1", Labels: map[string]string{v1beta1constants.LabelExtensionsAuthorizationAdditionalPermissions: "true"}}}
			clusterRole2 = &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "clusterRole2", Labels: map[string]string{v1beta1constants.LabelExtensionsAuthorizationAdditionalPermissions: "true"}}}
			clusterRole3 = &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "clusterRole3"}}

			Expect(fakeClient.Create(ctx, clusterRole1)).To(Succeed())
			Expect(fakeClient.Create(ctx, clusterRole2)).To(Succeed())
			Expect(fakeClient.Create(ctx, clusterRole3)).To(Succeed())
		})

		It("should map to all seeds", func() {
			Expect(reconciler.MapToAllClusterRoles(ctx, log, fakeClient, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterRole1.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterRole2.Name}},
			))
		})
	})
})
