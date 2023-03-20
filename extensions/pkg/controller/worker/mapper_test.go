// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker_test

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("Worker Mapper", func() {
	var (
		ctx  = context.TODO()
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

	Describe("#MachineSetToWorkerMapper", func() {
		var (
			resourceName = "machineSet"
			namespace    = "shoot"
		)

		It("should find all objects for the passed worker", func() {
			mapper := worker.MachineSetToWorkerMapper(nil)

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{
						Items: []extensionsv1alpha1.Worker{
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

			result := mapper.Map(ctx, logr.Discard(), c, &machinev1alpha1.MachineSet{
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
				mapper = worker.MachineSetToWorkerMapper(predicates)
			)

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{
						Items: []extensionsv1alpha1.Worker{
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

			result := mapper.Map(ctx, logr.Discard(), c, &machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			mapper := worker.MachineSetToWorkerMapper(nil)

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{}
					return nil
				})

			result := mapper.Map(ctx, logr.Discard(), c, &machinev1alpha1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because the passed object is no worker", func() {
			mapper := worker.MachineSetToWorkerMapper(nil)
			result := mapper.Map(ctx, logr.Discard(), c, &extensionsv1alpha1.Cluster{})
			Expect(result).To(BeNil())
		})
	})

	Describe("#MachineToWorkerMapper", func() {
		var (
			resourceName = "machineSet"
			namespace    = "shoot"
		)

		It("should find all objects for the passed worker", func() {
			mapper := worker.MachineToWorkerMapper(nil)

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{
						Items: []extensionsv1alpha1.Worker{
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

			result := mapper.Map(ctx, logr.Discard(), c, &machinev1alpha1.Machine{
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
				mapper = worker.MachineToWorkerMapper(predicates)
			)

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{
						Items: []extensionsv1alpha1.Worker{
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

			result := mapper.Map(ctx, logr.Discard(), c, &machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			mapper := worker.MachineToWorkerMapper(nil)

			c.EXPECT().
				List(
					gomock.Any(),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{}
					return nil
				})

			result := mapper.Map(ctx, logr.Discard(), c, &machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because the passed object is no worker", func() {
			mapper := worker.MachineToWorkerMapper(nil)

			result := mapper.Map(ctx, logr.Discard(), c, &extensionsv1alpha1.Cluster{})
			Expect(result).To(BeNil())
		})
	})
})
