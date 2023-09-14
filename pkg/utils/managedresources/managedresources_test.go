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
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

type errorClient struct {
	client.Client
	failSecretCreate bool
	failMRCreate     bool
	failMRPatch      bool
	failMRGet        bool
	err              error
}

func (e *errorClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	switch obj.(type) {
	case *resourcesv1alpha1.ManagedResource:
		if e.failMRGet {
			return e.err
		}
	}

	return e.Client.Get(ctx, key, obj, opts...)
}

func (e *errorClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	switch obj.(type) {
	case *corev1.Secret:
		if e.failSecretCreate {
			return e.err
		}
	case *resourcesv1alpha1.ManagedResource:
		if e.failMRCreate {
			return e.err
		}
	}

	return e.Client.Create(ctx, obj, opts...)
}

func (e *errorClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	switch obj.(type) {
	case *resourcesv1alpha1.ManagedResource:
		if e.failMRPatch {
			return e.err
		}
	}

	return e.Client.Patch(ctx, obj, patch, opts...)
}

var _ = Describe("managedresources", func() {
	var (
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

		fakeClient client.Client
		mr         *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		mr = &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}}
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
			errClient := &errorClient{err: fakeErr, failSecretCreate: true, Client: fakeClient}
			Expect(CreateForShoot(ctx, errClient, namespace, name, LabelValueGardener, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should return the error of the managed resource reconciliation", func() {
			errClient := &errorClient{err: fakeErr, failMRCreate: true, Client: fakeClient}
			Expect(CreateForShoot(ctx, errClient, namespace, name, LabelValueGardener, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should successfully create secret and managed resource", func() {
			secretName, _ := NewSecret(fakeClient, namespace, name, data, true)
			expectedMR := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ManagedResource",
					APIVersion: "resources.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs:   []corev1.LocalObjectReference{{Name: secretName}},
					KeepObjects:  pointer.Bool(keepObjects),
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				},
			}

			Expect(references.InjectAnnotations(expectedMR)).To(Succeed())

			Expect(CreateForShoot(ctx, fakeClient, namespace, name, LabelValueGardener, keepObjects, data)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			Expect(mr).To(Equal(expectedMR))

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret).To(Equal(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: pointer.Bool(true),
				Type:      corev1.SecretTypeOpaque,
			}))
		})
	})

	Describe("#DeleteForShoot", func() {
		It("should successfully delete all related resources", func() {
			secret1 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managedresource-" + name, Namespace: namespace}}
			secret2 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managedresource-" + name, Namespace: namespace}}
			Expect(kubernetesutils.MakeUnique(secret2)).To(Succeed())

			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					// Reference only the second secret
					// The delete function should delete both secrets for backwards compatible reasons
					SecretRefs: []corev1.LocalObjectReference{{Name: secret2.Name}},
				},
			}

			for _, o := range []client.Object{secret1, secret2, mr} {
				Expect(fakeClient.Create(ctx, o)).To(Succeed())
			}

			Expect(DeleteForShoot(ctx, fakeClient, namespace, name)).To(Succeed())

			for _, o := range []client.Object{secret1, secret2, mr} {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(o), o)).To(BeNotFoundError())
			}
		})
	})

	Describe("#CreateForSeed", func() {

		var (
			secretName string
			expectedMR *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			secretName, _ = NewSecret(fakeClient, namespace, name, data, true)
			expectedMR = &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ManagedResource",
					APIVersion: "resources.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs:  []corev1.LocalObjectReference{{Name: secretName}},
					KeepObjects: pointer.Bool(keepObjects),
					Class:       pointer.String("seed"),
				},
			}
		})

		It("should return the error of the secret reconciliation", func() {
			errClient := &errorClient{err: fakeErr, failSecretCreate: true, Client: fakeClient}
			Expect(CreateForSeed(ctx, errClient, namespace, name, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should return the error of the managed resource reconciliation", func() {
			errClient := &errorClient{err: fakeErr, failSecretCreate: true, Client: fakeClient}
			Expect(CreateForSeed(ctx, errClient, namespace, name, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should successfully create secret and managed resource", func() {
			Expect(references.InjectAnnotations(expectedMR)).To(Succeed())

			Expect(CreateForSeed(ctx, fakeClient, namespace, name, keepObjects, data)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			Expect(mr).To(Equal(expectedMR))

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret).To(Equal(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: pointer.Bool(true),
				Type:      corev1.SecretTypeOpaque,
			}))
		})

		It("should successfully create secret and managed resource if the namespace is 'shoot--foo--bar'", func() {
			namespace := "shoot--foo--bar"
			secretName, _ := NewSecret(fakeClient, namespace, name, data, true)
			expectedMR.Namespace = namespace
			expectedMR.Labels = nil
			expectedMR.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: secretName}}
			Expect(references.InjectAnnotations(expectedMR)).To(Succeed())

			mr.Namespace = namespace

			Expect(CreateForSeed(ctx, fakeClient, namespace, name, keepObjects, data)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			Expect(mr).To(Equal(expectedMR))

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret).To(Equal(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: pointer.Bool(true),
				Type:      corev1.SecretTypeOpaque,
			}))
		})
	})

	Describe("#DeleteForSeed", func() {
		It("should successfully delete all related resources", func() {
			secret1 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managedresource-" + name, Namespace: namespace}}
			secret2 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managedresource-" + name, Namespace: namespace}}
			Expect(kubernetesutils.MakeUnique(secret2)).To(Succeed())

			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					// Reference only the second secret
					// The delete function should delete both secrets for backwards compatible reasons
					SecretRefs: []corev1.LocalObjectReference{{Name: secret2.Name}},
				},
			}

			for _, o := range []client.Object{secret1, secret2, mr} {
				Expect(fakeClient.Create(ctx, o)).To(Succeed())
			}

			Expect(DeleteForSeed(ctx, fakeClient, namespace, name)).To(Succeed())

			for _, o := range []client.Object{secret1, secret2, mr} {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(o), o)).To(BeNotFoundError())
			}
		})
	})

	Describe("#SetKeepObjects", func() {
		It("should patch the managed resource", func() {
			mr := managedResource(false)
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			Expect(SetKeepObjects(ctx, fakeClient, namespace, name, true)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			Expect(*mr.Spec.KeepObjects).To(BeTrue())
		})

		It("should not fail if the managed resource is not found", func() {
			Expect(SetKeepObjects(ctx, fakeClient, namespace, name, true)).To(Succeed())
		})

		It("should fail if the managed resource could not be updated", func() {
			errClient := &errorClient{err: fakeErr, failMRPatch: true, Client: fakeClient}
			Expect(SetKeepObjects(ctx, errClient, namespace, name, true)).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitUntilHealthy", func() {
		It("should fail when the managed resource cannot be read", func() {
			errClient := &errorClient{err: fakeErr, failMRGet: true, Client: fakeClient}
			Expect(WaitUntilHealthy(ctx, errClient, namespace, name)).To(MatchError(fakeErr))
		})

		It("should retry when the managed resource is not healthy yet", func() {
			DeferCleanup(test.WithVar(&IntervalWait, time.Millisecond*10))

			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			shouldFail := atomic.Bool{}
			shouldFail.Store(true)
			go func() {
				defer GinkgoRecover()
				_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 1
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
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
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 1
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
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
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				shouldFail.Store(false)
			}()

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			Expect(WaitUntilHealthy(timeoutCtx, fakeClient, namespace, name)).To(Succeed())
			Expect(shouldFail.Load()).To(BeFalse()) // to ensure that the goroutine actually applies all patches
		})
	})

	Describe("#WaitUntilHealthyAndNotProgressing()", func() {
		It("should fail when the managed resource cannot be read", func() {
			errClient := &errorClient{err: fakeErr, failMRGet: true, Client: fakeClient}
			Expect(WaitUntilHealthyAndNotProgressing(ctx, errClient, namespace, name)).To(MatchError(fakeErr))
		})

		It("should retry when the managed resource is not healthy yet or still progressing", func() {
			DeferCleanup(test.WithVar(&IntervalWait, time.Millisecond*10))

			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			shouldFail := atomic.Bool{}
			shouldFail.Store(true)
			go func() {
				defer GinkgoRecover()
				_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 2
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 2,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 1
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
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
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 1
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
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
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				time.Sleep(time.Millisecond * 50)

				_, err = controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
					mr.ObjectMeta.Generation = 1
					mr.Status = resourcesv1alpha1.ManagedResourceStatus{
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					}
					return nil
				})
				Expect(err).To(Not(HaveOccurred()))
				shouldFail.Store(false)
			}()

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			Expect(WaitUntilHealthyAndNotProgressing(timeoutCtx, fakeClient, namespace, name)).To(Succeed())
			Expect(shouldFail.Load()).To(BeFalse()) // to ensure that the goroutine actually applies all patches
		})
	})

	Describe("#WaitUntilDeleted", func() {
		It("should not return error if managed resource and corresponding secret does not exist", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-secret",
					Namespace: namespace,
					Labels:    map[string]string{resourcesv1alpha1.ReferencedBy: "other-MR"},
				},
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			newContenxt, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			Expect(WaitUntilDeleted(newContenxt, fakeClient, namespace, name)).To(Succeed())
		})

		It("should not return error if managed resource and secret does not exist", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    map[string]string{resourcesv1alpha1.ReferencedBy: name},
				},
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			newContenxt, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			Expect(WaitUntilDeleted(newContenxt, fakeClient, namespace, name)).To(MatchError(ContainSubstring(fmt.Sprintf("resource(s) [%s] still exists", name))))
		})

		It("should return a severe error if managed resource retrieval fails", func() {
			errClient := &errorClient{err: fakeErr, failMRGet: true, Client: fakeClient}
			Expect(WaitUntilDeleted(ctx, errClient, namespace, name)).To(MatchError(fakeErr))
		})

		It("should return a generic timeout error if the resource does not get deleted in time", func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
			defer cancel()
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			Expect(WaitUntilDeleted(timeoutCtx, fakeClient, namespace, name)).To(MatchError(ContainSubstring(fmt.Sprintf("resource %s/%s still exists", namespace, name))))
		})

		It("should return a timeout error containing the resources which are blocking the deletion when the reason is DeletionFailed", func() {
			blockingResourcesMessage := "resource test-secret still exists"
			mr.Status = resourcesv1alpha1.ManagedResourceStatus{
				ObservedGeneration: 1,
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:    resourcesv1alpha1.ResourcesApplied,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  resourcesv1alpha1.ConditionDeletionFailed,
						Message: blockingResourcesMessage,
					},
				},
			}
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
			defer cancel()
			Expect(WaitUntilDeleted(timeoutCtx, fakeClient, namespace, name)).To(MatchError(ContainSubstring(blockingResourcesMessage)))
		})

		It("should return a timeout error containing the resources which are blocking the deletion when the reason is DeletionPending", func() {
			blockingResourcesMessage := "resource test-secret still exists"
			mr.Status = resourcesv1alpha1.ManagedResourceStatus{
				ObservedGeneration: 1,
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:    resourcesv1alpha1.ResourcesApplied,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  resourcesv1alpha1.ConditionDeletionPending,
						Message: blockingResourcesMessage,
					},
				},
			}
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
			defer cancel()
			Expect(WaitUntilDeleted(timeoutCtx, fakeClient, namespace, name)).To(MatchError(ContainSubstring(blockingResourcesMessage)))
		})
	})

	Describe("#CheckIfManagedResourcesExist", func() {
		var class = "foo"

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
