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

package internal_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/client/kubernetes/test"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PlantClientMap", func() {
	var (
		ctx              context.Context
		ctrl             *gomock.Controller
		c                *mockclient.MockClient
		fakeGardenClient *fakeclientset.ClientSet

		cm      clientmap.ClientMap
		key     clientmap.ClientSetKey
		factory *internal.PlantClientSetFactory

		plant *gardencorev1beta1.Plant
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeGardenClient = fakeclientset.NewClientSetBuilder().WithClient(c).Build()

		plant = &gardencorev1beta1.Plant{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "urban-gardening",
				Name:      "tulip",
			},
			Spec: gardencorev1beta1.PlantSpec{
				SecretRef: corev1.LocalObjectReference{
					Name: "tulip-secret",
				},
			},
		}

		key = keys.ForPlant(plant)

		factory = &internal.PlantClientSetFactory{
			GetGardenClient: func(ctx context.Context) (kubernetes.Interface, error) {
				return fakeGardenClient, nil
			},
		}
		cm = internal.NewPlantClientMap(factory, logger.NewNopLogger())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#GetClient", func() {
		It("should fail if ClientSetKey type is unsupported", func() {
			key = fakeKey{}
			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey")))
		})

		It("should fail if GetGardenClient fails", func() {
			fakeErr := fmt.Errorf("fake")
			factory.GetGardenClient = func(ctx context.Context) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("failed to get garden client: fake")))
		})

		It("should fail if it cannot get Plant object", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Plant{})).
				Return(apierrors.NewNotFound(gardencorev1beta1.Resource("plant"), plant.Name))

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("failed to get Plant object")))
		})

		It("should fail if Plant object does not have a secretRef", func() {
			plant.Spec.SecretRef = corev1.LocalObjectReference{}
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Plant{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					plant.DeepCopyInto(obj.(*gardencorev1beta1.Plant))
					return nil
				})

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("does not have a secretRef")))
		})

		It("should fail if NewClientFromSecret fails", func() {
			fakeErr := fmt.Errorf("fake")
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Plant{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					plant.DeepCopyInto(obj.(*gardencorev1beta1.Plant))
					return nil
				})
			internal.NewClientFromSecret = func(ctx context.Context, c client.Client, namespace, secretName string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: fake", key.Key())))
		})

		It("should correctly construct a new ClientSet", func() {
			fakeCS := fakeclientset.NewClientSet()
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Plant{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					plant.DeepCopyInto(obj.(*gardencorev1beta1.Plant))
					return nil
				}).Times(2)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Spec.SecretRef.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					return nil
				})
			internal.NewClientFromSecret = func(ctx context.Context, c client.Client, namespace, secretName string, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				Expect(c).To(BeIdenticalTo(fakeGardenClient.Client()))
				Expect(namespace).To(Equal(plant.Namespace))
				Expect(secretName).To(Equal(plant.Spec.SecretRef.Name))
				Expect(fns).To(ConsistOfConfigFuncs(
					kubernetes.WithClientOptions(client.Options{
						Scheme: kubernetes.PlantScheme,
					}),
				))
				return fakeCS, nil
			}

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#CalculateClientSetHash", func() {
		It("should fail if ClientSetKey type is unsupported", func() {
			key = fakeKey{}
			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey")))
		})

		It("should fail if getPlantSecretRef fails", func() {
			fakeErr := fmt.Errorf("fake")
			factory.GetGardenClient = func(ctx context.Context) (kubernetes.Interface, error) {
				return nil, fakeErr
			}

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("failed to get garden client: fake")))
		})

		It("should fail if Get Plant Secret fails", func() {
			fakeErr := fmt.Errorf("fake")
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Plant{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					plant.DeepCopyInto(obj.(*gardencorev1beta1.Plant))
					return nil
				})
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Spec.SecretRef.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					return fakeErr
				})

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(BeEmpty())
			Expect(err).To(MatchError("fake"))
		})

		It("should correctly calculate hash", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Plant{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					plant.DeepCopyInto(obj.(*gardencorev1beta1.Plant))
					return nil
				})
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: plant.Namespace, Name: plant.Spec.SecretRef.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					(&corev1.Secret{}).DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})

			hash, err := factory.CalculateClientSetHash(ctx, key)
			Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
