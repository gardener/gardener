// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package csinode_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/csinode"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
)

func TestCSINode(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Webhook CloudProvider Suite")
}

const (
	nodeFoo       = "shoot--foo"
	providerFoo   = "provider-foo"
	providerBar   = "provider-bar"
	driverFoo     = "foo.csi"
	driverBar     = "bar.csi"
	driverBaz     = "baz.csi"
	fooWorkerPool = "worker-foo"
)

var _ = Describe("Mutator", func() {
	var (
		mgr                        *mockmanager.MockManager
		ctrl                       *gomock.Controller
		cSeed                      *mockclient.MockClient
		cShoot                     *mockclient.MockClient
		logger                     logr.Logger
		args                       *csinode.Args
		mutFunc                    csinode.CSINodeMutateFunc
		worker                     *extensionsv1alpha1.Worker
		node                       *corev1.Node
		mutator                    webhook.MutatorWithShootClient
		csiNew, csiNewCopy, csiOld *storagev1.CSINode
		cluster                    *extensions.Cluster
		ctx                        context.Context
		allocatableCount           *int32
	)

	BeforeEach(func() {
		allocatableCount = pointer.Int32(10)
		ctrl = gomock.NewController(GinkgoT())
		cSeed = mockclient.NewMockClient(ctrl)
		cShoot = mockclient.NewMockClient(ctrl)

		logger = log.Log.WithName("test")
		ctx = context.Background()

		// Create fake manager
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(cSeed)
		mutFunc = csinode.GenericCSINodeMutate

		args = &csinode.Args{
			Provider: providerFoo,
			Drivers: map[string]csinode.CSINodeMutateFunc{
				driverFoo: mutFunc,
			},
		}

		mutator = csinode.NewMutator(mgr, logger, args)
		csiNew = &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeFoo,
			},
			Spec: storagev1.CSINodeSpec{
				Drivers: []storagev1.CSINodeDriver{
					{
						Name: driverFoo,
						Allocatable: &storagev1.VolumeNodeResources{
							Count: allocatableCount,
						},
					},
					{
						Name: driverBaz,
					},
				},
			},
		}
		csiNewCopy = csiNew.DeepCopy()

		csiOld = &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeFoo,
			},
			Spec: storagev1.CSINodeSpec{
				Drivers: []storagev1.CSINodeDriver{},
			},
		}

		cluster = &extensions.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: providerFoo,
					},
				},
			},
		}
		worker = &extensionsv1alpha1.Worker{
			Spec: extensionsv1alpha1.WorkerSpec{
				Pools: []extensionsv1alpha1.WorkerPool{
					extensionsv1alpha1.WorkerPool{
						Name: fooWorkerPool,
						DataVolumes: []extensionsv1alpha1.DataVolume{
							{
								Name: "foo1",
							},
							{
								Name: "foo2",
							},
						},
					},
				},
			},
		}
		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeFoo,
				Labels: map[string]string{
					constants.LabelWorkerPool: fooWorkerPool,
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("Should ignore shoots of other provider-types", func() {
		cluster.Shoot.Spec.Provider.Type = providerBar
		Expect(mutator.Mutate(ctx, csiNew, csiOld, cShoot, cluster)).To(Succeed())
		Expect(csiNew).To(Equal(csiNewCopy))
	})

	It("Should ignore drivers not registered with the args", func() {
		csiNew.Spec.Drivers[0].Name = driverBar
		csiNewCopy = csiNew.DeepCopy()

		Expect(mutator.Mutate(ctx, csiNew, csiOld, cShoot, cluster)).To(Succeed())
		Expect(csiNew).To(Equal(csiNewCopy))
	})
	It("Should not mutate if driver is already present", func() {
		csiOld = csiNew
		Expect(mutator.Mutate(ctx, csiNew, csiOld, cShoot, cluster)).To(Succeed())
		Expect(csiNew).To(Equal(csiNewCopy))
	})
	It("Should return an error if worker pool is not found", func() {
		node.Labels = map[string]string{}
		cShoot.EXPECT().Get(ctx, k8sclient.ObjectKey{
			Name: node.Name,
		}, gomock.AssignableToTypeOf(&corev1.Node{})).DoAndReturn(getter(node))
		Expect(mutator.Mutate(ctx, csiNew, csiOld, cShoot, cluster)).NotTo(Succeed())

	})
	It("Should mutate correctly", func() {
		cShoot.EXPECT().Get(ctx, k8sclient.ObjectKey{
			Name: node.Name,
		}, gomock.AssignableToTypeOf(&corev1.Node{})).DoAndReturn(getter(node))
		cSeed.EXPECT().Get(ctx, k8sclient.ObjectKey{
			Namespace: cluster.ObjectMeta.Name,
			Name:      cluster.Shoot.Name,
		}, gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).DoAndReturn(getter(worker))

		Expect(mutator.Mutate(ctx, csiNew, csiOld, cShoot, cluster)).To(Succeed())
		Expect(*csiNew.Spec.Drivers[0].Allocatable.Count).To(BeEquivalentTo(8))
	})
	It("Should skip registering the driver if node reached max attachments", func() {
		csiNew.Spec.Drivers[0].Allocatable.Count = pointer.Int32(2)
		cShoot.EXPECT().Get(ctx, k8sclient.ObjectKey{
			Name: node.Name,
		}, gomock.AssignableToTypeOf(&corev1.Node{})).DoAndReturn(getter(node))
		cSeed.EXPECT().Get(ctx, k8sclient.ObjectKey{
			Namespace: cluster.ObjectMeta.Name,
			Name:      cluster.Shoot.Name,
		}, gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).DoAndReturn(getter(worker))

		Expect(mutator.Mutate(ctx, csiNew, csiOld, cShoot, cluster)).To(Succeed())
		Expect(*csiNew.Spec.Drivers[0].Allocatable.Count).To(BeEquivalentTo(1))
	})
})

func getter[T any](t *T) func(context.Context, k8sclient.ObjectKey, *T, ...k8sclient.GetOption) error {
	return func(_ context.Context, _ k8sclient.ObjectKey, tIn *T, _ ...k8sclient.GetOption) error {
		*tIn = *t
		return nil
	}
}
