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

package managedresources_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/managedresources"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "test"
	name      = "managed-resource"
)

var _ = Describe("managedresources", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake")

		managedResource = func(keepObjects bool) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{
						{
							Name: name,
						},
					},
					KeepObjects: &keepObjects,
				},
			}
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
	})

	Describe("#KeepManagedResourceObjects", func() {
		It("should update the managed resource if the value of keepObjects is different", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
				DoAndReturn(clientGet(managedResource(false)))
			c.EXPECT().Update(ctx, managedResource(true)).Return(nil)

			err := KeepManagedResourceObjects(ctx, c, namespace, name, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not update the managed resource if the value of keepObjects is the same", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
				DoAndReturn(clientGet(managedResource(true)))

			err := KeepManagedResourceObjects(ctx, c, namespace, name, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not fail if the managed resource is not found", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			err := KeepManagedResourceObjects(ctx, c, namespace, name, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if the managed resource could not be updated", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
				DoAndReturn(clientGet(managedResource(false)))
			c.EXPECT().Update(ctx, managedResource(true)).Return(errors.New("error"))

			err := KeepManagedResourceObjects(ctx, c, namespace, name, true)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#WaitUntilManagedResourceHealthy", func() {
		It("should fail when the managed resource cannot be read", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(WaitUntilManagedResourceHealthy(ctx, c, namespace, name)).To(MatchError(fakeErr))
		})

		It("should retry when the managed resource is not healthy yet", func() {
			oldInterval := IntervalWait
			defer func() { IntervalWait = oldInterval }()
			IntervalWait = time.Millisecond

			gomock.InOrder(
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 2,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
					},
				})),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 2,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
					},
				})),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 2,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
						Conditions: []resourcesv1alpha1.ManagedResourceCondition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: resourcesv1alpha1.ConditionTrue,
							},
						},
					},
				})),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 2,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
						Conditions: []resourcesv1alpha1.ManagedResourceCondition{
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: resourcesv1alpha1.ConditionTrue,
							},
						},
					},
				})),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []resourcesv1alpha1.ManagedResourceCondition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: resourcesv1alpha1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: resourcesv1alpha1.ConditionFalse,
							},
						},
					},
				})),
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []resourcesv1alpha1.ManagedResourceCondition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: resourcesv1alpha1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: resourcesv1alpha1.ConditionTrue,
							},
						},
					},
				})),
			)

			Expect(WaitUntilManagedResourceHealthy(ctx, c, namespace, name)).To(Succeed())
		})
	})
})

func clientGet(managedResource *resourcesv1alpha1.ManagedResource) interface{} {
	return func(_ context.Context, _ client.ObjectKey, mr *resourcesv1alpha1.ManagedResource) error {
		*mr = *managedResource
		return nil
	}
}
