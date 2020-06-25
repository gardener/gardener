// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerregistration

import (
	"context"
	"errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

var _ = Describe("Controller", func() {
	logger.Logger = logger.NewNopLogger()

	var (
		gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory

		queue                           *fakeQueue
		controllerRegistrationSeedQueue *fakeQueue
		c                               *Controller

		controllerRegistrationName = "controllerRegistration"
	)

	BeforeEach(func() {
		gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		controllerRegistrationInformer := gardenCoreInformerFactory.Core().V1beta1().ControllerRegistrations()
		controllerRegistrationLister := controllerRegistrationInformer.Lister()
		seedInformer := gardenCoreInformerFactory.Core().V1beta1().Seeds()
		seedLister := seedInformer.Lister()

		queue = &fakeQueue{}
		controllerRegistrationSeedQueue = &fakeQueue{}

		c = &Controller{
			controllerRegistrationQueue:     queue,
			controllerRegistrationLister:    controllerRegistrationLister,
			controllerRegistrationSeedQueue: controllerRegistrationSeedQueue,
			seedLister:                      seedLister,
		}
	})

	Describe("#controllerRegistrationAdd", func() {
		It("should do nothing because the object key computation fails", func() {
			obj := "foo"

			c.controllerRegistrationAdd(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
			}

			c.controllerRegistrationAdd(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(controllerRegistrationName))
		})

		It("should add the object to the queue and not enqueue any seeds due to list error", func() {
			obj := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
			}
			c.seedLister = newFakeSeedLister(c.seedLister, nil, nil, errors.New("err"))

			c.controllerRegistrationAdd(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(controllerRegistrationName))
		})

		It("should add the object to the queue and enqueue all seeds", func() {
			obj := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
			}

			var (
				seed1 = "seed1"
				seed2 = "seed2"
			)
			seedList := []*gardencorev1beta1.Seed{
				{ObjectMeta: metav1.ObjectMeta{Name: seed1}},
				{ObjectMeta: metav1.ObjectMeta{Name: seed2}},
			}
			c.seedLister = newFakeSeedLister(c.seedLister, nil, seedList, nil)

			c.controllerRegistrationAdd(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(controllerRegistrationName))
			Expect(controllerRegistrationSeedQueue.Len()).To(Equal(len(seedList)))
			Expect(controllerRegistrationSeedQueue.items[0]).To(Equal(seed1))
			Expect(controllerRegistrationSeedQueue.items[1]).To(Equal(seed2))
		})
	})

	Describe("#controllerRegistrationUpdate", func() {
		It("should do nothing because the object key computation fails", func() {
			obj := "foo"

			c.controllerRegistrationUpdate(nil, obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
			}

			c.controllerRegistrationUpdate(nil, obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(controllerRegistrationName))
		})
	})

	Describe("#controllerRegistrationDelete", func() {
		It("should do nothing because the object key computation fails", func() {
			obj := "foo"

			c.controllerRegistrationDelete(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue (tomb stone)", func() {
			obj := cache.DeletedFinalStateUnknown{
				Key: controllerRegistrationName,
			}

			c.controllerRegistrationDelete(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(controllerRegistrationName))
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
			}

			c.controllerRegistrationDelete(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(controllerRegistrationName))
		})
	})

	Describe("#reconcileControllerRegistrationKey", func() {
		It("should return an error because the key cannot be split", func() {
			Expect(c.reconcileControllerRegistrationKey("a/b/c")).To(HaveOccurred())
		})

		It("should return nil because object not found", func() {
			c.controllerRegistrationLister = newFakeControllerRegistrationLister(c.controllerRegistrationLister, nil, apierrors.NewNotFound(schema.GroupResource{}, controllerRegistrationName))

			Expect(c.reconcileControllerRegistrationKey(controllerRegistrationName)).NotTo(HaveOccurred())
		})

		It("should return err because object not found", func() {
			err := errors.New("error")

			c.controllerRegistrationLister = newFakeControllerRegistrationLister(c.controllerRegistrationLister, nil, err)

			Expect(c.reconcileControllerRegistrationKey(controllerRegistrationName)).To(Equal(err))
		})

		It("should return the result of the reconciliation (nil)", func() {
			obj := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
			}

			c.controllerRegistrationControl = &fakeControllerRegistrationControl{}
			c.controllerRegistrationLister = newFakeControllerRegistrationLister(c.controllerRegistrationLister, obj, nil)

			Expect(c.reconcileControllerRegistrationKey(controllerRegistrationName)).To(BeNil())
		})

		It("should return the result of the reconciliation (error)", func() {
			obj := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerRegistrationName,
				},
			}

			c.controllerRegistrationControl = &fakeControllerRegistrationControl{result: errors.New("")}
			c.controllerRegistrationLister = newFakeControllerRegistrationLister(c.controllerRegistrationLister, obj, nil)

			Expect(c.reconcileControllerRegistrationKey(controllerRegistrationName)).To(HaveOccurred())
		})
	})
})

type fakeControllerRegistrationControl struct {
	result error
}

func (f *fakeControllerRegistrationControl) Reconcile(obj *gardencorev1beta1.ControllerRegistration) error {
	return f.result
}

type fakeControllerRegistrationLister struct {
	gardencorelisters.ControllerRegistrationLister

	getResult *gardencorev1beta1.ControllerRegistration
	getErr    error
}

func newFakeControllerRegistrationLister(controllerRegistrationLister gardencorelisters.ControllerRegistrationLister, getResult *gardencorev1beta1.ControllerRegistration, getErr error) *fakeControllerRegistrationLister {
	return &fakeControllerRegistrationLister{
		ControllerRegistrationLister: controllerRegistrationLister,

		getResult: getResult,
		getErr:    getErr,
	}
}

func (c *fakeControllerRegistrationLister) Get(string) (*gardencorev1beta1.ControllerRegistration, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return c.getResult, nil
}

var _ = Describe("ControllerRegistrationControl", func() {
	var (
		ctrl                   *gomock.Controller
		clientMap              clientmap.ClientMap
		k8sGardenRuntimeClient *mockclient.MockClient

		gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory

		d *defaultControllerRegistrationControl

		ctx                        = context.TODO()
		controllerRegistrationName = "controllerRegistration"
		obj                        *gardencorev1beta1.ControllerRegistration
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		k8sGardenClient := fakeclientset.NewClientSetBuilder().WithClient(k8sGardenRuntimeClient).WithDirectClient(k8sGardenRuntimeClient).Build()

		clientMap = fake.NewClientMap().AddClient(keys.ForGarden(), k8sGardenClient)

		gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		controllerInstallationInformer := gardenCoreInformerFactory.Core().V1beta1().ControllerInstallations()
		controllerInstallationLister := controllerInstallationInformer.Lister()

		d = &defaultControllerRegistrationControl{clientMap, controllerInstallationLister}
		obj = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: controllerRegistrationName,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		Context("deletion timestamp not set", func() {
			It("should ensure the finalizer (error)", func() {
				err := apierrors.NewNotFound(schema.GroupResource{}, controllerRegistrationName)

				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(err)

				Expect(d.Reconcile(obj)).To(HaveOccurred())
			})

			It("should ensure the finalizer (no error)", func() {
				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(nil)
				k8sGardenRuntimeClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(nil)

				Expect(d.Reconcile(obj)).NotTo(HaveOccurred())
			})
		})

		Context("deletion timestamp set", func() {
			BeforeEach(func() {
				now := metav1.Now()
				obj.DeletionTimestamp = &now
				obj.Finalizers = []string{FinalizerName}
			})

			It("should do nothing because finalizer is not present", func() {
				obj.Finalizers = nil

				Expect(d.Reconcile(obj)).NotTo(HaveOccurred())
			})

			It("should return an error because installation list failed", func() {
				err := errors.New("err")

				d.controllerInstallationLister = newFakeControllerInstallationLister(d.controllerInstallationLister, nil, err)

				Expect(d.Reconcile(obj)).To(Equal(err))
			})

			It("should return an error because installation referencing controllerRegistration exists", func() {
				controllerInstallationList := []*gardencorev1beta1.ControllerInstallation{
					{
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{
								Name: controllerRegistrationName,
							},
						},
					},
				}

				d.controllerInstallationLister = newFakeControllerInstallationLister(d.controllerInstallationLister, controllerInstallationList, nil)

				err := d.Reconcile(obj)
				Expect(err.Error()).To(ContainSubstring("cannot remove finalizer"))
			})

			It("should remove the finalizer (error)", func() {
				err := errors.New("some err")
				d.controllerInstallationLister = newFakeControllerInstallationLister(d.controllerInstallationLister, nil, nil)

				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(err)

				Expect(d.Reconcile(obj)).To(HaveOccurred())
			})

			It("should remove the finalizer (no error)", func() {
				d.controllerInstallationLister = newFakeControllerInstallationLister(d.controllerInstallationLister, nil, nil)

				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(nil)
				k8sGardenRuntimeClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(nil)
				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(controllerRegistrationName), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistration{})).Return(nil)

				Expect(d.Reconcile(obj)).NotTo(HaveOccurred())
			})
		})
	})
})
