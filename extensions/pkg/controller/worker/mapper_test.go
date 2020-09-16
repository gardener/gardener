// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ = Describe("Worker Mapper", func() {
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

	Describe("#MachineSetToWorkerMapper", func() {
		var (
			resourceName = "machineSet"
			namespace    = "shoot"
		)

		It("should find all objects for the passed worker", func() {
			mapper := worker.MachineSetToWorkerMapper(nil)
			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.AssignableToTypeOf(context.TODO()),
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

			result := mapper.Map(handler.MapObject{
				Object: &machinev1alpha1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
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
			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.AssignableToTypeOf(context.TODO()),
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

			result := mapper.Map(handler.MapObject{
				Object: &machinev1alpha1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			mapper := worker.MachineSetToWorkerMapper(nil)

			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.AssignableToTypeOf(context.TODO()),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{}
					return nil
				})

			result := mapper.Map(handler.MapObject{
				Object: &machinev1alpha1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because the passed object is no worker", func() {
			mapper := worker.MachineSetToWorkerMapper(nil)
			result := mapper.Map(handler.MapObject{
				Object: &extensionsv1alpha1.Cluster{},
			})
			ExpectInject(inject.ClientInto(c, mapper))
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
			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.AssignableToTypeOf(context.TODO()),
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

			result := mapper.Map(handler.MapObject{
				Object: &machinev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
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
			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.AssignableToTypeOf(context.TODO()),
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

			result := mapper.Map(handler.MapObject{
				Object: &machinev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			mapper := worker.MachineToWorkerMapper(nil)

			ExpectInject(inject.ClientInto(c, mapper))

			c.EXPECT().
				List(
					gomock.AssignableToTypeOf(context.TODO()),
					gomock.AssignableToTypeOf(&extensionsv1alpha1.WorkerList{}),
					gomock.AssignableToTypeOf(client.InNamespace(namespace)),
				).
				DoAndReturn(func(_ context.Context, actual *extensionsv1alpha1.WorkerList, _ ...client.ListOption) error {
					*actual = extensionsv1alpha1.WorkerList{}
					return nil
				})

			result := mapper.Map(handler.MapObject{
				Object: &machinev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				},
			})

			Expect(result).To(BeEmpty())
		})

		It("should find no objects because the passed object is no worker", func() {
			mapper := worker.MachineToWorkerMapper(nil)
			result := mapper.Map(handler.MapObject{
				Object: &extensionsv1alpha1.Cluster{},
			})
			ExpectInject(inject.ClientInto(c, mapper))
			Expect(result).To(BeNil())
		})
	})
})

func ExpectInject(ok bool, err error) {
	Expect(err).NotTo(HaveOccurred())
	Expect(ok).To(BeTrue(), "no injection happened")
}
