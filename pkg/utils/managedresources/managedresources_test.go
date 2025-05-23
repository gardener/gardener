// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresources_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
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
		ctx     = context.Background()
		fakeErr = errors.New("fake")

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

		fakeOps   *retryfake.Ops
		resetVars func()
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		mr = &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}}

		fakeOps = &retryfake.Ops{MaxAttempts: 1}
		resetVars = test.WithVars(
			&retry.Until, fakeOps.Until,
		)
	})

	AfterEach(func() {
		resetVars()
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
			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, actual)).To(Succeed())
			Expect(actual).To(Equal(&resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Labels:          map[string]string{"origin": origin},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  ptr.To(keepObjects),
				},
			}))
		})
	})

	Describe("#Update", func() {
		It("should fail to update managed resource because it doesn't exist", func() {
			Expect(Update(ctx, fakeClient, namespace, name, nil, false, "", data, nil, nil, nil)).To(BeNotFoundError())

			// Even though the update of the managed resource is expected to fail,
			// the secret is still created because it is immutable.
			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList, client.Limit(1))).To(Succeed())
			Expect(secretList.Items[0].Name).To(HavePrefix(name))

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
		})
	})

	Describe("#CreateForShoot{WithLabels}", func() {
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
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs:   []corev1.LocalObjectReference{{Name: secretName}},
					KeepObjects:  ptr.To(keepObjects),
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
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: ptr.To(true),
				Type:      corev1.SecretTypeOpaque,
			}))
		})

		It("should successfully create secret and managed resource with labels", func() {
			labels := map[string]string{"foo": "bar"}

			secretName, _ := NewSecret(fakeClient, namespace, name, data, true)
			expectedMR := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          utils.MergeStringMaps(map[string]string{"origin": "gardener"}, labels),
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs:   []corev1.LocalObjectReference{{Name: secretName}},
					KeepObjects:  ptr.To(keepObjects),
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				},
			}

			Expect(references.InjectAnnotations(expectedMR)).To(Succeed())

			Expect(CreateForShootWithLabels(ctx, fakeClient, namespace, name, LabelValueGardener, keepObjects, labels, data)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			Expect(mr).To(Equal(expectedMR))

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: ptr.To(true),
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

	Describe("#CreateForSeed{WithLabels}", func() {
		var (
			secretName string
			expectedMR *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			secretName, _ = NewSecret(fakeClient, namespace, name, data, true)
			expectedMR = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs:  []corev1.LocalObjectReference{{Name: secretName}},
					KeepObjects: ptr.To(keepObjects),
					Class:       ptr.To("seed"),
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
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: ptr.To(true),
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
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: ptr.To(true),
				Type:      corev1.SecretTypeOpaque,
			}))
		})

		It("should successfully create secret and managed resource with labels", func() {
			labels := map[string]string{"foo": "bar"}
			expectedMR.Labels = utils.MergeStringMaps(expectedMR.Labels, labels)

			Expect(references.InjectAnnotations(expectedMR)).To(Succeed())

			Expect(CreateForSeedWithLabels(ctx, fakeClient, namespace, name, keepObjects, labels, data)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			Expect(mr).To(Equal(expectedMR))

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
				},
				Data:      data,
				Immutable: ptr.To(true),
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

		It("should return error when the managed resource is not healthy yet (observed generation does not match)", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 2
				mr.Status = resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				}
				return nil
			})
			Expect(err).To(Not(HaveOccurred()))

			Expect(WaitUntilHealthy(ctx, fakeClient, namespace, name)).To(MatchError(ContainSubstring("managed resource test/managed-resource is not healthy")))
		})

		It("should return error when the managed resource is not healthy yet (ResourcesApplied is not true)", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 1
				mr.Status = resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionFalse,
						},
					},
				}

				return nil
			})
			Expect(err).To(Not(HaveOccurred()))

			Expect(WaitUntilHealthy(ctx, fakeClient, namespace, name)).To(MatchError(ContainSubstring("managed resource test/managed-resource is not healthy")))
		})

		It("should return error when the managed resource is not healthy yet (ResourcesHealthy is not true)", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 1
				mr.Status = resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
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

			Expect(WaitUntilHealthy(ctx, fakeClient, namespace, name)).To(MatchError(ContainSubstring("managed resource test/managed-resource is not healthy")))
		})

		It("should succeed when the managed resource is healthy", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 1
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

			Expect(WaitUntilHealthy(ctx, fakeClient, namespace, name)).To(Succeed())
		})
	})

	Describe("#WaitUntilHealthyAndNotProgressing", func() {
		It("should fail when the managed resource cannot be read", func() {
			errClient := &errorClient{err: fakeErr, failMRGet: true, Client: fakeClient}
			Expect(WaitUntilHealthyAndNotProgressing(ctx, errClient, namespace, name)).To(MatchError(fakeErr))
		})

		It("should return error when the managed resource is not healthy yet (observed generation does not match)", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 2
				mr.Status = resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				}
				return nil
			})
			Expect(err).To(Not(HaveOccurred()))

			Expect(WaitUntilHealthyAndNotProgressing(ctx, fakeClient, namespace, name)).To(MatchError(ContainSubstring("managed resource test/managed-resource is not healthy")))
		})

		It("should return error when the managed resource is not healthy yet (ResourcesApplied is not true)", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 1
				mr.Status = resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionFalse,
						},
					},
				}

				return nil
			})
			Expect(err).To(Not(HaveOccurred()))

			Expect(WaitUntilHealthyAndNotProgressing(ctx, fakeClient, namespace, name)).To(MatchError(ContainSubstring("managed resource test/managed-resource is not healthy")))
		})

		It("should return error when the managed resource is not healthy yet (ResourcesHealthy is not true)", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 1
				mr.Status = resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
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

			Expect(WaitUntilHealthyAndNotProgressing(ctx, fakeClient, namespace, name)).To(MatchError(ContainSubstring("managed resource test/managed-resource is not healthy")))
		})

		It("should return error when the managed resource is not healthy yet (ResourcesProgressing is not false)", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 1
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
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				}

				return nil
			})
			Expect(err).To(Not(HaveOccurred()))

			Expect(WaitUntilHealthyAndNotProgressing(ctx, fakeClient, namespace, name)).To(MatchError(ContainSubstring("managed resource test/managed-resource is still progressing")))
		})

		It("should succeed when the managed resource is healthy and not progressing", func() {
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, fakeClient, mr, func() error {
				mr.Generation = 1
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

			Expect(WaitUntilHealthyAndNotProgressing(ctx, fakeClient, namespace, name)).To(Succeed())
		})
	})

	Describe("#WaitUntilDeleted", func() {
		It("should not return error if managed resource does not exist", func() {
			Expect(WaitUntilDeleted(ctx, fakeClient, namespace, name)).To(Succeed())
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
				obj.Spec.Class = ptr.To("bar")
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

	Describe("#GetObjects", func() {
		var (
			scheme       = runtime.NewScheme()
			serial       = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
			codecFactory = serializer.NewCodecFactory(scheme)

			registry *Registry
		)

		BeforeEach(func() {
			Expect(kubernetesscheme.AddToScheme(scheme)).To(Succeed())

			registry = NewRegistry(scheme, codecFactory, serial)
		})

		It("should return all objects", func() {
			expectedObjects := []client.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
					Data: map[string]string{"foo": "bar"},
				},
				&corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Namespace",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   "bar",
						Labels: map[string]string{"foo": "bar"},
					},
				},
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foobar",
						Namespace: "bar",
					},
				},
			}

			By("Create managed resource with objects")
			resources, err := registry.AddAllAndSerialize(expectedObjects...)
			Expect(err).ToNot(HaveOccurred())
			Expect(CreateForSeed(ctx, fakeClient, namespace, name, false, resources)).To(Succeed())

			By("Get objects from managed resource")
			objects, err := GetObjects(ctx, fakeClient, namespace, name)
			Expect(err).ToNot(HaveOccurred())
			Expect(objects).To(DeepEqual(expectedObjects))
		})
	})
})
