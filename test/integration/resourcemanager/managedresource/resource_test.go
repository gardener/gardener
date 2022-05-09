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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
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

const (
	secretName    = "test-secret"
	configMapName = "test-configmap"
	dataKey       = "configmap.yaml"
)

var _ = Describe("ManagedResource controller tests", func() {
	var (
		secretForManagedResource *corev1.Secret
		managedResource          *resourcesv1alpha1.ManagedResource

		configMap *corev1.ConfigMap
	)

	Context("create, update and delete operations", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: testNamespace.Name,
				},
			}

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: testNamespace.Name,
				},
				Data: secretDataForObject(configMap, dataKey),
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(filter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
			}).Should(Succeed())

			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		})

		Context("create managed resource", func() {
			It("should successfully create the resources and maintain proper status conditions", func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

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

			It("should fail to create the resource due to missing secret reference", func() {
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionFalse), withReason("CannotReadSecret")),
				)
			})

			It("should fail to create the resource due to incorrect object", func() {
				newConfigMap := &corev1.ConfigMap{}
				secretForManagedResource.Data = secretDataForObject(newConfigMap, dataKey)

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionApplyFailed)),
				)
			})

			It("should correctly set the condition ResourceApplied to Progressing", func() {
				// this finalizer is added to prolong the deletion of the resource so that we can
				// observe the controller successfully setting ResourceApplied condition to Progressing
				configMap.Finalizers = append(configMap.Finalizers, "kubernetes")
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

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
						Name:      "new-configmap",
						Namespace: testNamespace.Name,
					},
				}

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data = secretDataForObject(newConfigMap, dataKey)
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionProgressing), withReason(resourcesv1alpha1.ConditionDeletionPending)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Or(Succeed(), BeNoMatchError()))
				configMap.Finalizers = []string{}
				Expect(testClient.Update(ctx, configMap)).To(Succeed())
			})
		})

		Context("update managed resource", func() {
			const newDataKey = "secret.yaml"
			var newResource *corev1.Secret

			BeforeEach(func() {
				newResource = &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "update-test-secret",
						Namespace: testNamespace.Name,
					},
					Data: map[string][]byte{
						"entry": []byte("value"),
					},
					Type: corev1.SecretTypeOpaque,
				}

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			It("should successfully create a new resource when the secret referenced by the managed resource is updated with data containing the new resource", func() {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data[newDataKey] = secretDataForObject(newResource, newDataKey)[newDataKey]
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

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

			It("should successfully update the managed resource with a new secret reference", func() {
				newSecretForManagedResource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "new-secret",
						Namespace: testNamespace.Name,
					},
					Data: secretDataForObject(newResource, dataKey),
				}

				Expect(testClient.Create(ctx, newSecretForManagedResource)).To(Succeed())
				managedResource.Spec.SecretRefs = append(managedResource.Spec.SecretRefs, corev1.LocalObjectReference{Name: newSecretForManagedResource.Name})
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

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

			It("should fail to update the managed resource if a new incorrect resource is added to the secret referenced by the managed resource", func() {
				newResource.TypeMeta = metav1.TypeMeta{}

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data[newDataKey] = secretDataForObject(newResource, newDataKey)[newDataKey]
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionApplyFailed)),
				)
			})
		})

		Context("delete managed resource", func() {
			BeforeEach(func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.SetFinalizers([]string{"test-finalizer"})
				Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())
			})

			JustAfterEach(func() {
				patch := client.MergeFrom(managedResource.DeepCopy())
				controllerutil.RemoveFinalizer(managedResource, "test-finalizer")
				Expect(testClient.Patch(ctx, managedResource, patch))
			})

			It("should set ManagedResource to unhealthy", func() {
				Expect(testClient.Delete(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionDeletionPending)),
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionDeletionPending)),
				)
			})
		})
	})

	Context("Resource class", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: testNamespace.Name,
				},
			}

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: testNamespace.Name,
				},
				Data: secretDataForObject(configMap, dataKey),
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String("test"),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
			}).Should(Succeed())

			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		})

		It("should not reconcile ManagedResource of any other class except the default class", func() {
			Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())

			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(BeNotFoundError())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).ShouldNot(
				containCondition(ofType(resourcesv1alpha1.ResourcesApplied)),
			)
		})
	})

	Context("Reconciliation Modes/Annotations", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: testNamespace.Name,
				},
			}

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: testNamespace.Name,
				},
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(filter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}

			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				}).Should(Succeed())

				Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		Context("Ignore Mode", func() {
			It("should not update/re-apply resources having ignore mode annotation and remove them from the ManagedResource status", func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

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

		Context("Delete On Invalid Update", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"})
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			It("should not delete the resource on valid update", func() {
				configMap.Data = map[string]string{"foo": "bar"}

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				}).Should(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			It("should delete the resource on invalid update", func() {
				newConfigMap := configMap.DeepCopy()
				newConfigMap.TypeMeta = metav1.TypeMeta{}

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data = secretDataForObject(newConfigMap, dataKey)
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionApplyFailed)),
				)

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())
				}).Should(Succeed())
			})
		})

		Context("Keep Object", func() {
			BeforeEach(func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.KeepObject: "true"})

				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)
			})

			It("should keep the object in case it is removed from the MangedResource", func() {
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{}
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

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
				Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				}).Should(Succeed())

				Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			})
		})

		Context("Ignore", func() {
			It("should not revert any manual update on resource managed by ManagedResource when resource itself has ignore annotation", func() {
				configMap.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})

				secretForManagedResource.Data = secretDataForObject(configMap, dataKey)

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())

				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Update(ctx, configMap)).To(Succeed())

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

			It("should not revert any manual update on the resources when the ManagedResource has ignore annotation", func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				managedResource.SetAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
					containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionFalse), withReason(resourcesv1alpha1.ConditionManagedResourceIgnored)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				configMap.Data = map[string]string{"foo": "bar"}
				Expect(testClient.Update(ctx, configMap)).To(Succeed())

				Consistently(func(g Gomega) map[string]string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					return configMap.Data
				}).Should(HaveKeyWithValue("foo", "bar"))
			})
		})
	})

	Context("Preserve Replica/Resource", func() {
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
					Name:      "test-deploy",
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

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: testNamespace.Name,
				},
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(filter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))

			// wait for finalizer to be added
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				return deployment.Finalizers
			}).Should(ConsistOf("foregroundDeletion"))

			// remove finalizer so the deployment can be deleted
			Expect(controllerutils.PatchRemoveFinalizers(ctx, testClient, deployment, "foregroundDeletion")).To(BeNil())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(BeNotFoundError())
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			}).Should(Succeed())

			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		})

		Context("Preserve Replicas", func() {
			It("should not preserve changes in the number of replicas if the resource don't have preserve-replicas annotation", func() {
				secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Replicas = pointer.Int32Ptr(5)
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				Eventually(func(g Gomega) int32 {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return *deployment.Spec.Replicas
				}).Should(BeEquivalentTo(1))
			})

			It("should preserve changes in the number of replicas if the resource has preserve-replicas annotation", func() {
				deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveReplicas: "true"})
				secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Replicas = pointer.Int32Ptr(5)
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				Consistently(func(g Gomega) int32 {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return *deployment.Spec.Replicas
				}).Should(BeEquivalentTo(5))
			})
		})

		Context("Preserve Resources", func() {
			var (
				newPodTemplateSpec *corev1.PodTemplateSpec
			)
			BeforeEach(func() {
				defaultPodTemplateSpec.Spec.Containers[0].Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("25m"),
						corev1.ResourceMemory: resource.MustParse("25Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				}

				deployment.Spec.Template = *defaultPodTemplateSpec

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

			It("should not preserve changes in resource requests and limits in Pod if the resource doesn't have preserve-resources annotation", func() {
				secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template = *newPodTemplateSpec
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				Eventually(func(g Gomega) corev1.ResourceRequirements {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Resources
				}).Should(DeepEqual(defaultPodTemplateSpec.Spec.Containers[0].Resources))
			})

			It("should preserve changes in resource requests and limits in Pod if the resource has preserve-resources annotation", func() {
				deployment.SetAnnotations(map[string]string{resourcesv1alpha1.PreserveResources: "true"})
				secretForManagedResource.Data = secretDataForObject(deployment, "deployment.yaml")

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesApplied), withStatus(gardencorev1beta1.ConditionTrue), withReason(resourcesv1alpha1.ConditionApplySucceeded)),
				)

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				updatedDeployment := deployment.DeepCopy()
				updatedDeployment.Spec.Template = *newPodTemplateSpec
				Expect(testClient.Update(ctx, updatedDeployment)).To(Succeed())

				Eventually(func(g Gomega) corev1.ResourceRequirements {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Resources
				}).ShouldNot(DeepEqual(defaultPodTemplateSpec.Spec.Containers[0].Resources))
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
