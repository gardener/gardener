// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedresource_test

import (
	"encoding/json"

	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ManagedResource controller tests", func() {
	const (
		dataKey       = "configmap.yaml"
		testFinalizer = "resources.gardener.cloud/test-finalizer"
	)

	var (
		// resourceName is used as a base name for all resources in the current test case
		resourceName string
		objectKey    client.ObjectKey

		secretForManagedResource *corev1.Secret
		managedResource          *resourcesv1alpha1.ManagedResource

		configMap *corev1.ConfigMap
	)

	BeforeEach(func() {
		// use unique resource names specific to test case
		// If we use the same names for all test cases, failing cases will cause cascading failures because of AlreadyExistsErrors.
		// We want to avoid cascading failures because they are distracting from the actual root failure, and we want to
		// test all cases in a clean environment to make sure we don't have any unintended dependencies between test code.
		// We could also use GenerateName for this purpose, but then we would need an extra call to update the name of the
		// marshalled ConfigMap in the already created Secret.
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(CurrentSpecReport().LeafNodeLocation.String()))[:8]
		objectKey = client.ObjectKey{Namespace: testNamespace.Name, Name: resourceName}

		configMap = &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
		}

		secretForManagedResource = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Data: secretDataForObject(configMap, dataKey),
		}

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:      pointer.String(filter.ResourceClass()),
				SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
			},
		}
	})

	JustBeforeEach(func() {
		if secretForManagedResource != nil {
			By("creating Secret for test")
			log.Info("Creating Secret for test", "secret", objectKey)
			Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
		}

		By("creating ManagedResource for test")
		log.Info("Creating ManagedResource for test", "managedResource", objectKey)
		Expect(testClient.Create(ctx, managedResource)).To(Succeed())
	})

	AfterEach(func() {
		By("deleting ManagedResource")
		Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, objectKey, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError(), "ManagedResource should get released")
			g.Expect(testClient.Get(ctx, objectKey, &corev1.ConfigMap{})).To(BeNotFoundError(), "managed ConfigMap should get deleted")
		}).Should(Succeed())

		if secretForManagedResource != nil {
			By("deleting Secret")
			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		}
	})

	Describe("create managed resource", func() {
		It("should successfully create the resources and maintain proper status conditions", func() {
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
			)
		})

		Context("missing secret", func() {
			BeforeEach(func() {
				secretForManagedResource = nil
			})

			It("should fail to create the resource due to missing secret reference", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionFalse), withReason("CannotReadSecret")),
				)
			})
		})

		Context("missing TypeMeta in object", func() {
			BeforeEach(func() {
				newConfigMap := &corev1.ConfigMap{}
				secretForManagedResource.Data = secretDataForObject(newConfigMap, dataKey)
			})

			It("should fail to create the resource due to incorrect object", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionApplyFailed)),
				)
			})
		})
	})

	Describe("update managed resource", func() {
		const newDataKey = "secret.yaml"
		var newResource *corev1.Secret

		BeforeEach(func() {
			newResource = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName + "-new",
					Namespace: testNamespace.Name,
				},
				Data: map[string][]byte{
					"entry": []byte("value"),
				},
				Type: corev1.SecretTypeOpaque,
			}
		})

		JustBeforeEach(func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
			)
		})

		Describe("resource set changes", func() {
			BeforeEach(func() {
				// this finalizer is added to prolong the deletion of the resource so that we can
				// observe the controller successfully setting ResourceApplied condition to Progressing
				controllerutil.AddFinalizer(configMap, testFinalizer)
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			It("should correctly set the condition ResourceApplied to Progressing", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				newConfigMap := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName + "-new",
						Namespace: testNamespace.Name,
					},
				}

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data = secretDataForObject(newConfigMap, dataKey)
				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionProgressing), withReason(resourcesv1alpha1.ConditionDeletionPending)),
				)

				patch = client.MergeFrom(configMap.DeepCopy())
				controllerutil.RemoveFinalizer(configMap, testFinalizer)
				Expect(testClient.Patch(ctx, configMap, patch)).To(Succeed())
			})
		})

		Describe("new resource added", func() {
			It("should successfully create a new resource", func() {
				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data[newDataKey] = secretDataForObject(newResource, newDataKey)[newDataKey]
				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)).To(Succeed())
				}).Should(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			It("should fail to update the managed resource if a new incorrect resource is added", func() {
				newResource.TypeMeta = metav1.TypeMeta{}

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data[newDataKey] = secretDataForObject(newResource, newDataKey)[newDataKey]
				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionApplyFailed)),
				)
			})
		})

		Describe("new secret reference", func() {
			It("should successfully update the managed resource with a new secret reference", func() {
				newSecretForManagedResource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName + "-new-resource",
						Namespace: testNamespace.Name,
					},
					Data: secretDataForObject(newResource, dataKey),
				}

				Expect(testClient.Create(ctx, newSecretForManagedResource)).To(Succeed())

				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.Spec.SecretRefs = append(managedResource.Spec.SecretRefs, corev1.LocalObjectReference{Name: newSecretForManagedResource.Name})
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)).To(Succeed())
				}).Should(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})
		})
	})

	Describe("delete managed resource", func() {
		JustBeforeEach(func() {
			// add finalizer to prolong deletion of ManagedResource after resource-manager removed its finalizer
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, managedResource)).To(Succeed())
				g.Expect(controllerutils.PatchAddFinalizers(ctx, testClient, managedResource, testFinalizer)).To(Succeed())
			}).Should(Succeed())
		})

		JustAfterEach(func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, objectKey, managedResource)).To(Succeed())
				g.Expect(controllerutils.PatchRemoveFinalizers(ctx, testClient, managedResource, testFinalizer)).To(Succeed())
			}).Should(Succeed())
		})

		It("should set ManagedResource to unhealthy", func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
			)

			By("deleting ManagedResource")
			Expect(testClient.Delete(ctx, managedResource)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionProgressing), withReason(resourcesv1alpha1.ConditionDeletionPending)),
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionDeletionPending)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionDeletionPending)),
			)
		})
	})

	Describe("Resource class", func() {
		BeforeEach(func() {
			managedResource.Spec.Class = pointer.String("test")
		})

		It("should not reconcile ManagedResource of any other class except the default class", func() {
			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(BeNotFoundError())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(BeEmpty())
		})
	})

	Describe("Reconciliation Modes/Annotations", func() {
		Describe("Ignore Mode", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			It("should not update/re-apply resources having ignore mode annotation and remove them from the ManagedResource status", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
				}).Should(Succeed())
			})
		})

		Describe("Delete On Invalid Update", func() {
			var originalUID types.UID

			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"})
				// provoke invalid update by trying to update an immutable configmap's data
				configMap.Immutable = pointer.Bool(true)
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			JustBeforeEach(func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				cm := configMap.DeepCopy() // copy in order to not remove TypeMeta from configMap which we rely on later
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
				// use UID to distinguish between object instances with different names
				// i.e. if UID changed, the object got deleted and recreated, if not it wasn't deleted
				originalUID = cm.UID
				Expect(originalUID).NotTo(BeEmpty()) // sanity check
			})

			It("should not delete the resource on valid update", func() {
				metav1.SetMetaDataLabel(&configMap.ObjectMeta, "foo", "bar")

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Consistently(func(g Gomega) types.UID {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.UID
				}).Should(Equal(originalUID), "ConfigMap should not get deleted")
			})

			It("should delete the resource on invalid update", func() {
				configMap.Data = map[string]string{"invalid": "update"}

				patch := client.MergeFrom(secretForManagedResource.DeepCopy())
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
				Expect(testClient.Patch(ctx, secretForManagedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) types.UID {
					// also accept transient NotFoundError because controller needs to delete the ConfigMap first before recreating it
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Or(Succeed(), BeNotFoundError()))
					return configMap.UID
				}).ShouldNot(Equal(originalUID), "ConfigMap should get deleted and recreated")
			})
		})

		Describe("Keep Object", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.KeepObject: "true"})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			JustBeforeEach(func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			JustAfterEach(func() {
				Expect(testClient.Delete(ctx, configMap)).To(Or(Succeed(), BeNotFoundError()))
			})

			It("should keep the object in case it is removed from the MangedResource", func() {
				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{}
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				}).Should(Succeed())
			})

			It("should keep the object even after deletion of ManagedResource", func() {
				By("deleting ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				}).Should(Succeed())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			})
		})

		Describe("Ignore on Resource", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
			})

			It("should not revert any manual update to managed resource", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				patch := client.MergeFrom(configMap.DeepCopy())
				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Patch(ctx, configMap, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Consistently(func(g Gomega) map[string]string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data
				}).Should(HaveKeyWithValue("foo", "bar"))
			})
		})

		Describe("Ignore on ManagedResource", func() {
			It("should not revert any manual update to the resources", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
					containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
				)

				patch = client.MergeFrom(configMap.DeepCopy())
				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Patch(ctx, configMap, patch)).To(Succeed())

				Consistently(func(g Gomega) map[string]string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data
				}).Should(HaveKeyWithValue("foo", "bar"))
			})
		})

		Describe("Preserve Replica/Resource", func() {
			var (
				defaultPodTemplateSpec *corev1.PodTemplateSpec
				deployment             *appsv1.Deployment
			)

			BeforeEach(func() {
				defaultPodTemplateSpec = &corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"foo": "bar"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "foo-container",
								Image: "foo",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("25m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
								},
							},
						},
					},
				}

				deployment = &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: testNamespace.Name,
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						Replicas: pointer.Int32Ptr(1),
						Template: *defaultPodTemplateSpec,
					},
				}

				secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")
			})

			AfterEach(func() {
				By("deleting ManagedResource")
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

				// resource-manager deletes Deployments with foreground deletion, which causes API server to add the
				// foregroundDeletion finalizer. It is removed by kube-controller-manager's garbage collector, which is not
				// running in envtest, so we me might need to remove it ourselves.
				Eventually(func(g Gomega) bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
					if apierrors.IsNotFound(err) {
						// deployment is gone, done
						return true
					}
					g.Expect(err).To(Succeed())
					// no point in checking whether finalizer is present, just try to remove it until Deployment is gone
					g.Expect(controllerutils.PatchRemoveFinalizers(ctx, testClient, deployment, metav1.FinalizerDeleteDependents)).To(Succeed())
					return false
				}).Should(BeTrue())
			})

			Describe("Preserve Replicas", func() {
				Context("resource doesn't have preserve-replicas annotation", func() {
					It("should not preserve changes in the number of replicas", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Replicas = pointer.Int32Ptr(5)
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						Eventually(func(g Gomega) int32 {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return *deployment.Spec.Replicas
						}).Should(BeEquivalentTo(1))
					})
				})

				Context("resource has preserve-replicas annotation", func() {
					BeforeEach(func() {
						deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveReplicas: "true"})
						secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")
					})

					It("should preserve changes in the number of replicas if the resource", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Replicas = pointer.Int32Ptr(5)
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						Consistently(func(g Gomega) int32 {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return *deployment.Spec.Replicas
						}).Should(BeEquivalentTo(5))
					})
				})
			})

			Describe("Preserve Resources", func() {
				var newPodTemplateSpec *corev1.PodTemplateSpec

				BeforeEach(func() {
					newPodTemplateSpec = defaultPodTemplateSpec.DeepCopy()
					newPodTemplateSpec.Spec.Containers[0].Resources = corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("35m"),
							corev1.ResourceMemory: resource.MustParse("35Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("60m"),
							corev1.ResourceMemory: resource.MustParse("60Mi"),
						},
					}
				})

				Context("resource doesn't have preserve-resources annotation", func() {
					It("should not preserve changes in resource requests and limits in Pod", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Template = *newPodTemplateSpec
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						Eventually(func(g Gomega) corev1.ResourceRequirements {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return deployment.Spec.Template.Spec.Containers[0].Resources
						}).Should(DeepEqual(defaultPodTemplateSpec.Spec.Containers[0].Resources))
					})
				})

				Context("resource has preserve-resources annotation", func() {
					BeforeEach(func() {
						deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveResources: "true"})
						secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")
					})

					It("should preserve changes in resource requests and limits in Pod", func() {
						Eventually(func(g Gomega) []gardencorev1beta1.Condition {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
							return managedResource.Status.Conditions
						}).Should(
							containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
						)

						patch := client.MergeFrom(deployment.DeepCopy())
						deployment.Spec.Template = *newPodTemplateSpec
						Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())

						Eventually(func(g Gomega) corev1.ResourceRequirements {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
							return deployment.Spec.Template.Spec.Containers[0].Resources
						}).ShouldNot(DeepEqual(defaultPodTemplateSpec.Spec.Containers[0].Resources))
					})
				})
			})
		})
	})
})

func secretDataForObject(obj runtime.Object, key string) map[string][]byte {
	jsonObject, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return map[string][]byte{key: jsonObject}
}

func containCondition(matchers ...gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return ContainElement(And(matchers...))
}

func ofType(conditionType gardencorev1beta1.ConditionType) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Type": Equal(conditionType),
	})
}

func withStatus(status gardencorev1beta1.ConditionStatus) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Status": Equal(status),
	})
}

func withReason(reason string) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Reason": Equal(reason),
	})
}
