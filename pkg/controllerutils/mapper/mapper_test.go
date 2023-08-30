// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package mapper

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler Suite")
}

var _ = Describe("Controller Mapper", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client
		ctrl       *gomock.Controller
		cache      *mockcache.MockCache
		mgr        *mockmanager.MockManager

		namespace = "some-namespace"
		cluster   *extensionsv1alpha1.Cluster

		newObjListFunc func() client.ObjectList
		infra          *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		ctrl = gomock.NewController(GinkgoT())
		cache = mockcache.NewMockCache(ctrl)
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetCache().Return(cache).AnyTimes()

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		newObjListFunc = func() client.ObjectList { return &extensionsv1alpha1.InfrastructureList{} }
		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "infra",
				Namespace: namespace,
			},
		}
	})

	Describe("#ClusterToObjectMapper", func() {
		var mapper Mapper

		BeforeEach(func() {
			mapper = ClusterToObjectMapper(mgr, newObjListFunc, nil)
		})

		It("should find all objects for the passed cluster", func() {
			Expect(fakeClient.Create(ctx, infra)).To(Succeed())

			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, cluster)).To(ConsistOf(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      infra.Name,
					Namespace: infra.Namespace,
				},
			}))
		})

		It("should find no objects for the passed cluster because predicates do not match", func() {
			predicates := []predicate.Predicate{
				predicate.Funcs{
					GenericFunc: func(event event.GenericEvent) bool {
						return false
					},
				},
			}
			mapper = ClusterToObjectMapper(mgr, newObjListFunc, predicates)

			Expect(fakeClient.Create(ctx, infra)).To(Succeed())

			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, cluster)).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, cluster)).To(BeEmpty())
		})

		It("should find no objects because the passed object is no cluster", func() {
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, infra)).To(BeEmpty())
		})
	})
})
