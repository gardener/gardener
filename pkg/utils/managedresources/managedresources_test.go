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

package managedresources_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/managedresources"
)

var _ = Describe("managedresources", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake")

		namespace   = "test"
		name        = "managed-resource"
		keepObjects = true
		data        = map[string][]byte{"some": []byte("data")}

		managedResource = func(keepObjects bool) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					KeepObjects: &keepObjects,
				},
			}
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
	})

	Describe("#NewForShoot", func() {
		It("should create a managed resource builder", func() {
			var (
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

				origin      = "foo-origin"
				keepObjects = true
			)

			managedResource := NewForShoot(fakeClient, namespace, name, origin, keepObjects)
			Expect(managedResource.Reconcile(ctx)).To(Succeed())

			actual := &resourcesv1alpha1.ManagedResource{}
			Expect(fakeClient.Get(ctx, kubernetesutils.Key(namespace, name), actual)).To(Succeed())
			Expect(actual).To(Equal(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resources.gardener.cloud/v1alpha1",
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Labels:          map[string]string{"origin": origin},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  pointer.Bool(keepObjects),
				},
			}))
		})
	})

	Describe("#CreateForShoot", func() {
		It("should return the error of the secret reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(CreateForShoot(ctx, c, namespace, name, LabelValueGardener, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should return the error of the managed resource reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(CreateForShoot(ctx, c, namespace, name, LabelValueGardener, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should successfully create secret and managed resource", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managedresource-" + name,
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: data,
				}),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Labels:    map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs:   []corev1.LocalObjectReference{{Name: "managedresource-" + name}},
						KeepObjects:  pointer.Bool(keepObjects),
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					},
				}),
			)

			Expect(CreateForShoot(ctx, c, namespace, name, LabelValueGardener, keepObjects, data)).To(Succeed())
		})
	})

	Describe("#DeleteForShoot", func() {
		var (
			secret          = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "managedresource-" + name}}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		It("should fail when the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
			)

			Expect(DeleteForShoot(ctx, c, namespace, name)).To(MatchError(fakeErr))
		})

		It("should fail when the secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret).Return(fakeErr),
			)

			Expect(DeleteForShoot(ctx, c, namespace, name)).To(MatchError(fakeErr))
		})

		It("should successfully delete all related resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret),
			)

			Expect(DeleteForShoot(ctx, c, namespace, name)).To(Succeed())
		})
	})

	Describe("#CreateForSeed", func() {
		It("should return the error of the secret reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(CreateForSeed(ctx, c, namespace, name, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should return the error of the managed resource reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(CreateForSeed(ctx, c, namespace, name, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should successfully create secret and managed resource", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managedresource-" + name,
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: data,
				}),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Labels:    map[string]string{"gardener.cloud/role": "seed-system-component"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs:  []corev1.LocalObjectReference{{Name: "managedresource-" + name}},
						KeepObjects: pointer.Bool(keepObjects),
						Class:       pointer.String("seed"),
					},
				}),
			)

			Expect(CreateForSeed(ctx, c, namespace, name, keepObjects, data)).To(Succeed())
		})

		It("should successfully create secret and managed resource if the namespace is 'shoot--foo--bar'", func() {
			namespace := "shoot--foo--bar"

			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managedresource-" + name,
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: data,
				}),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs:  []corev1.LocalObjectReference{{Name: "managedresource-" + name}},
						KeepObjects: pointer.Bool(keepObjects),
						Class:       pointer.String("seed"),
					},
				}),
			)

			Expect(CreateForSeed(ctx, c, namespace, name, keepObjects, data)).To(Succeed())
		})
	})

	Describe("#DeleteForSeed", func() {
		var (
			secret          = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "managedresource-" + name}}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		It("should fail when the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
			)

			Expect(DeleteForSeed(ctx, c, namespace, name)).To(MatchError(fakeErr))
		})

		It("should fail when the secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret).Return(fakeErr),
			)

			Expect(DeleteForSeed(ctx, c, namespace, name)).To(MatchError(fakeErr))
		})

		It("should successfully delete all related resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret),
			)

			Expect(DeleteForSeed(ctx, c, namespace, name)).To(Succeed())
		})
	})

	Describe("#SetKeepObjects", func() {
		It("should patch the managed resource", func() {
			c.EXPECT().Patch(ctx, managedResource(true), gomock.Any())

			err := SetKeepObjects(ctx, c, namespace, name, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not fail if the managed resource is not found", func() {
			c.EXPECT().Patch(ctx, managedResource(true), gomock.Any()).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			err := SetKeepObjects(ctx, c, namespace, name, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if the managed resource could not be updated", func() {
			c.EXPECT().Patch(ctx, managedResource(true), gomock.Any()).
				Return(errors.New("error"))

			err := SetKeepObjects(ctx, c, namespace, name, true)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#WaitUntilHealthy", func() {
		It("should fail when the managed resource cannot be read", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(WaitUntilHealthy(ctx, c, namespace, name)).To(MatchError(fakeErr))
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
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
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
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
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
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
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
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})),
			)

			Expect(WaitUntilHealthy(ctx, c, namespace, name)).To(Succeed())
		})
	})

	Describe("WaitUntilDeleted", func() {
		It("should not return error if managed resource does not exist", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, name))
			Expect(WaitUntilDeleted(ctx, c, namespace, name)).To(Succeed())
		})

		It("should return a severe error if managed resource retrieval fails", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)
			Expect(WaitUntilDeleted(ctx, c, namespace, name)).To(MatchError(fakeErr))
		})

		It("should return a generic timeout error if the resource does not get deleted in time", func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
			defer cancel()
			c.EXPECT().Get(timeoutCtx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).AnyTimes()
			Expect(WaitUntilDeleted(timeoutCtx, c, namespace, name)).To(MatchError(ContainSubstring(fmt.Sprintf("resource %s/%s still exists", namespace, name))))
		})

		It("should return a timeout error containing the resources which are blocking the deletion when the reason is DeletionFailed", func() {
			blockingResourcesMessage := "resource test-secret still exists"
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
			defer cancel()
			c.EXPECT().Get(timeoutCtx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:    resourcesv1alpha1.ResourcesApplied,
								Status:  gardencorev1beta1.ConditionFalse,
								Reason:  resourcesv1alpha1.ConditionDeletionFailed,
								Message: blockingResourcesMessage,
							},
						},
					},
				})).AnyTimes()
			Expect(WaitUntilDeleted(timeoutCtx, c, namespace, name)).To(MatchError(ContainSubstring(blockingResourcesMessage)))
		})

		It("should return a timeout error containing the resources which are blocking the deletion when the reason is DeletionPending", func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
			defer cancel()
			blockingResourcesMessage := "resource test-secret still exists"
			c.EXPECT().Get(timeoutCtx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				clientGet(&resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:    resourcesv1alpha1.ResourcesApplied,
								Status:  gardencorev1beta1.ConditionFalse,
								Reason:  resourcesv1alpha1.ConditionDeletionPending,
								Message: blockingResourcesMessage,
							},
						},
					},
				})).AnyTimes()
			Expect(WaitUntilDeleted(timeoutCtx, c, namespace, name)).To(MatchError(ContainSubstring(blockingResourcesMessage)))
		})
	})

	Describe("#CheckIfManagedResourcesExist", func() {
		var (
			fakeClient client.Client
			class      = "foo"
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		})

		Context("w/o class", func() {
			It("should return false because no resources exist", func() {
				resourcesExist, err := CheckIfManagedResourcesExist(ctx, fakeClient, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(resourcesExist).To(BeFalse())
			})

			It("should return false because existing resources have a class", func() {
				obj := managedResource(false)
				obj.Spec.Class = &class
				Expect(fakeClient.Create(ctx, obj)).To(Succeed())

				resourcesExist, err := CheckIfManagedResourcesExist(ctx, fakeClient, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(resourcesExist).To(BeFalse())
			})

			It("should return true because resources exist", func() {
				Expect(fakeClient.Create(ctx, managedResource(false))).To(Succeed())

				resourcesExist, err := CheckIfManagedResourcesExist(ctx, fakeClient, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(resourcesExist).To(BeTrue())
			})
		})

		Context("w/ class", func() {
			It("should return false because no resources exist", func() {
				resourcesExist, err := CheckIfManagedResourcesExist(ctx, fakeClient, &class)
				Expect(err).NotTo(HaveOccurred())
				Expect(resourcesExist).To(BeFalse())
			})

			It("should return false because existing resources have another class", func() {
				obj := managedResource(false)
				obj.Spec.Class = pointer.String("bar")
				Expect(fakeClient.Create(ctx, obj)).To(Succeed())

				resourcesExist, err := CheckIfManagedResourcesExist(ctx, fakeClient, &class)
				Expect(err).NotTo(HaveOccurred())
				Expect(resourcesExist).To(BeFalse())
			})

			It("should return true because resources exist", func() {
				obj := managedResource(false)
				obj.Spec.Class = &class
				Expect(fakeClient.Create(ctx, obj)).To(Succeed())

				resourcesExist, err := CheckIfManagedResourcesExist(ctx, fakeClient, &class)
				Expect(err).NotTo(HaveOccurred())
				Expect(resourcesExist).To(BeTrue())
			})
		})
	})
})

func clientGet(managedResource *resourcesv1alpha1.ManagedResource) interface{} {
	return func(_ context.Context, _ client.ObjectKey, mr *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
		*mr = *managedResource
		return nil
	}
}
