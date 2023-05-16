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
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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

		fakeClient       client.Client
		secret1          *corev1.Secret
		secret2          *corev1.Secret
		expectedChecksum = "d285aee1a9342ca3b8c7758589bda8dd7714a4e809ab95d333e54d3e3fed39bd"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		secret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"foo1": []byte("bar1"),
				"foo2": []byte("bar2"),
			},
		}
		secret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"foo2": []byte("bar2"),
			},
		}
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
			class = "foo"
		)

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

	Describe("#ComputeSecretsDataChecksum", func() {
		It("should compute the correct checksum", func() {
			secrets := []*corev1.Secret{secret1, secret2}

			Expect(ComputeSecretsDataChecksum(secrets)).To(Equal(expectedChecksum))
		})

		It("should compute the same checksum for both secrets slices", func() {
			one := []*corev1.Secret{secret1, secret2}
			two := []*corev1.Secret{
				{
					Data: map[string][]byte{
						"foo2": []byte("bar2"),
						"foo1": []byte("bar1"),
					},
				},
				{
					Data: map[string][]byte{
						"foo2": []byte("bar2"),
					},
				},
			}

			Expect(ComputeSecretsDataChecksum(one)).To(Equal(ComputeSecretsDataChecksum(two)))
		})

		It("should compute different checksums for the secrets slices", func() {
			one := []*corev1.Secret{secret2, secret1}
			two := []*corev1.Secret{secret1, secret2}

			Expect(ComputeSecretsDataChecksum(one)).ToNot(Equal(ComputeSecretsDataChecksum(two)))
		})
	})

	Describe("#ComputeExpectedSecretsDataChecksum", func() {
		var (
			managedResource *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name,
					Namespace:  namespace,
					Generation: 1,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{
						{Name: secret1.Name},
						{Name: secret2.Name},
					},
					KeepObjects: pointer.Bool(false),
				},
				Status: resourcesv1alpha1.ManagedResourceStatus{},
			}

		})

		It("should fail when a referenced secret is not found", func() {
			Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
			checksum, err := ComputeExpectedSecretsDataChecksum(ctx, fakeClient, managedResource)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			Expect(checksum).To(Equal(""))
		})

		It("should compute the correct expected checksum", func() {
			Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())
			checksum, err := ComputeExpectedSecretsDataChecksum(ctx, fakeClient, managedResource)
			Expect(err).ToNot(HaveOccurred())
			Expect(checksum).To(Equal(expectedChecksum))
		})
	})

	Describe("#WaitUntilHealthy", func() {
		var (
			managedResource *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name,
					Namespace:  namespace,
					Generation: 1,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{
						{Name: secret1.Name},
						{Name: secret2.Name},
					},
					KeepObjects: pointer.Bool(false),
				},
			}
		})

		It("should fail when the managed resource is not found", func() {
			err := WaitUntilHealthy(ctx, fakeClient, namespace, name)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should fail when a referenced secret is not found", func() {
			Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

			err := WaitUntilHealthy(ctx, fakeClient, namespace, name)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should eventually report healty managed resource", func() {
			oldInterval := IntervalWait
			IntervalWait = time.Millisecond * 10
			defer func() { IntervalWait = oldInterval }()

			Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

			finalStatus := resourcesv1alpha1.ManagedResourceStatus{
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
				SecretsDataChecksum: &expectedChecksum,
			}

			statuses := []resourcesv1alpha1.ManagedResourceStatus{
				{
					ObservedGeneration: 1,
				},
				{
					ObservedGeneration: 2,
				},
				{
					ObservedGeneration: 2,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
				{
					ObservedGeneration: 2,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
				{
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
				{
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
				finalStatus,
			}

			go func(statuses []resourcesv1alpha1.ManagedResourceStatus) {
				defer GinkgoRecover()
				for _, status := range statuses {
					time.Sleep(IntervalWait * 5) // give some time so that the wait function can retry a couple of times
					patch := client.MergeFrom(managedResource.DeepCopy())
					managedResource.Status = status
					Expect(fakeClient.Patch(ctx, managedResource, patch)).To(Succeed())
				}
			}(statuses)

			cancelCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			Expect(WaitUntilHealthy(cancelCtx, fakeClient, namespace, name)).To(Succeed())

			fetchedManagedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), fetchedManagedResource)).To(Succeed())
			Expect(fetchedManagedResource.Status).To(DeepEqual(finalStatus))
		})
	})
})

func clientGet(managedResource *resourcesv1alpha1.ManagedResource) interface{} {
	return func(_ context.Context, _ client.ObjectKey, mr *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
		*mr = *managedResource
		return nil
	}
}
