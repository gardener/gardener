// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	"context"
	"errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
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

		queue *fakeQueue
		c     *Controller

		seedName = "seed"
	)

	BeforeEach(func() {
		gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		seedInformer := gardenCoreInformerFactory.Core().V1beta1().Seeds()
		seedLister := seedInformer.Lister()

		queue = &fakeQueue{}
		c = &Controller{
			seedQueue:  queue,
			seedLister: seedLister,
		}
	})

	Describe("#seedAdd", func() {
		It("should do nothing because the object key computation fails", func() {
			obj := "foo"

			c.seedAdd(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			c.seedAdd(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})
	})

	Describe("#seedUpdate", func() {
		It("should do nothing because the object key computation fails", func() {
			obj := "foo"

			c.seedUpdate(nil, obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			c.seedUpdate(nil, obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})
	})

	Describe("#seedDelete", func() {
		It("should do nothing because the object key computation fails", func() {
			obj := "foo"

			c.seedDelete(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue (tomb stone)", func() {
			obj := cache.DeletedFinalStateUnknown{
				Key: seedName,
			}

			c.seedDelete(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			c.seedDelete(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})
	})

	Describe("#reconcileSeedKey", func() {
		It("should return an error because the key cannot be split", func() {
			Expect(c.reconcileSeedKey("a/b/c")).To(HaveOccurred())
		})

		It("should return nil because object not found", func() {
			c.seedLister = newFakeSeedLister(c.seedLister, nil, nil, apierrors.NewNotFound(schema.GroupResource{}, seedName))

			Expect(c.reconcileSeedKey(seedName)).NotTo(HaveOccurred())
		})

		It("should return err because object not found", func() {
			err := errors.New("error")

			c.seedLister = newFakeSeedLister(c.seedLister, nil, nil, err)

			Expect(c.reconcileSeedKey(seedName)).To(Equal(err))
		})

		It("should return the result of the reconciliation (nil)", func() {
			obj := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			c.seedControl = &fakeSeedControl{}
			c.seedLister = newFakeSeedLister(c.seedLister, obj, nil, nil)

			Expect(c.reconcileSeedKey(seedName)).To(BeNil())
		})

		It("should return the result of the reconciliation (error)", func() {
			obj := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			c.seedControl = &fakeSeedControl{result: errors.New("")}
			c.seedLister = newFakeSeedLister(c.seedLister, obj, nil, nil)

			Expect(c.reconcileSeedKey(seedName)).To(HaveOccurred())
		})
	})
})

type fakeSeedControl struct {
	result error
}

func (f *fakeSeedControl) Reconcile(obj *gardencorev1beta1.Seed) error {
	return f.result
}

var _ = Describe("SeedControl", func() {
	var (
		ctrl                   *gomock.Controller
		clientMap              clientmap.ClientMap
		k8sGardenRuntimeClient *mockclient.MockClient

		gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory

		d *defaultSeedControl

		ctx      = context.TODO()
		seedName = "seed"
		obj      *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		k8sGardenClient := fakeclientset.NewClientSetBuilder().WithClient(k8sGardenRuntimeClient).WithDirectClient(k8sGardenRuntimeClient).Build()

		clientMap = fake.NewClientMap().AddClient(keys.ForGarden(), k8sGardenClient)

		gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		controllerInstallationInformer := gardenCoreInformerFactory.Core().V1beta1().ControllerInstallations()
		controllerInstallationLister := controllerInstallationInformer.Lister()

		d = &defaultSeedControl{clientMap, controllerInstallationLister}
		obj = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		Context("deletion timestamp not set", func() {
			It("should ensure the finalizer (error)", func() {
				err := apierrors.NewNotFound(schema.GroupResource{}, seedName)

				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(err)

				Expect(d.Reconcile(obj)).To(HaveOccurred())
			})

			It("should ensure the finalizer (no error)", func() {
				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(nil)
				k8sGardenRuntimeClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(nil)

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

			It("should return an error because installation referencing seed exists", func() {
				controllerInstallationList := []*gardencorev1beta1.ControllerInstallation{
					{
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							SeedRef: corev1.ObjectReference{
								Name: seedName,
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

				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(err)

				Expect(d.Reconcile(obj)).To(HaveOccurred())
			})

			It("should remove the finalizer (no error)", func() {
				d.controllerInstallationLister = newFakeControllerInstallationLister(d.controllerInstallationLister, nil, nil)

				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(nil)
				k8sGardenRuntimeClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(nil)
				k8sGardenRuntimeClient.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).Return(nil)

				Expect(d.Reconcile(obj)).NotTo(HaveOccurred())
			})
		})
	})
})
