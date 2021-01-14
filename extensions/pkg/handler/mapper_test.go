// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package handler

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler Suite")
}

var _ = Describe("Controller Mapper", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ClusterToObjectMapper", func() {
		var (
			resourceName = "infra"
			namespace    = "shoot"

			newObjListFunc = func() client.ObjectList { return &extensionsv1alpha1.InfrastructureList{} }
		)

		It("should find all objects for the passed cluster", func() {
			mapper := ClusterToObjectMapper(newObjListFunc, nil)
			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.InfrastructureList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.InfrastructureList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.InfrastructureList{
						Items: []extensionsv1alpha1.Infrastructure{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      resourceName,
									Namespace: namespace,
								},
							},
						},
					}
					return nil
				})

			result := mapper.Map(&extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			Expect(result).To(ConsistOf(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      resourceName,
					Namespace: namespace,
				},
			}))
		})

		It("should find no objects for the passed cluster because predicates do not match", func() {
			var (
				predicates = []predicate.Predicate{
					predicate.Funcs{
						GenericFunc: func(event event.GenericEvent) bool {
							return false
						},
					},
				}
				mapper = ClusterToObjectMapper(newObjListFunc, predicates)
			)
			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.InfrastructureList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.InfrastructureList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.InfrastructureList{
						Items: []extensionsv1alpha1.Infrastructure{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      resourceName,
									Namespace: namespace,
								},
							},
						},
					}
					return nil
				})

			result := mapper.Map(&extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			mapper := ClusterToObjectMapper(newObjListFunc, nil)

			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.InfrastructureList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.InfrastructureList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.InfrastructureList{}
					return nil
				})

			result := mapper.Map(&extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because the passed object is no cluster", func() {
			mapper := ClusterToObjectMapper(newObjListFunc, nil)
			result := mapper.Map(&extensionsv1alpha1.Infrastructure{})
			ExpectInject(inject.ClientInto(c, mapper))
			Expect(result).To(BeNil())
		})
	})
})

func ExpectInject(ok bool, err error) {
	Expect(err).NotTo(HaveOccurred())
	Expect(ok).To(BeTrue(), "no injection happened")
}
