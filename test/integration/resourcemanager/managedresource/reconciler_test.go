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

package reconciler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerv1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	secretName      = "test-secret"
	configMapName   = "test-configmap"
	newResourceName = "update-test-secret"
)

var _ = Describe("ManagedResource controller tests", func() {
	var (
		ctx                      = context.Background()
		secretForManagedResource *corev1.Secret
		managedResource          *resourcesv1alpha1.ManagedResource
		configMap                *corev1.ConfigMap
	)

	Context("create, update and delete operations", func() {
		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				}, ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				}}

			data, err := createSecretDataFromObject(configMap, "config-map.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeNil())

			secretForManagedResource = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespaceName,
				},
				Data: data,
			}

			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-managedresource",
					Namespace: namespaceName,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(filter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{Name: secretForManagedResource.Name}},
				},
			}
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
			}, time.Minute, 5*time.Second).Should(BeNotFoundError())
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}, time.Minute, 5*time.Second).Should(BeNotFoundError())
			Expect(testClient.Delete(ctx, secretForManagedResource)).To(Or(Succeed(), BeNotFoundError()))
		})

		Context("create managed resource", func() {
			It("should successfully create the resources and maintain proper status conditions", func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
				}, time.Minute, time.Second).Should(Succeed())

				Eventually(func() bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return err == nil && condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should fail to create the resource due to missing secret reference", func() {
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func() bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return err == nil && condition != nil && condition.Status == gardencorev1beta1.ConditionFalse && condition.Reason == "CannotReadSecret"
				}, 30*time.Second, time.Second).Should(BeTrue())
			})

			It("should fail to create the resource due to incorrect object", func() {
				configMap := &corev1.ConfigMap{}
				data, err := createSecretDataFromObject(configMap, "config-map.yaml")
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())
				secretForManagedResource.Data = data

				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func() bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return err == nil && condition != nil && condition.Status == gardencorev1beta1.ConditionFalse
				}, 30*time.Second, time.Second).Should(BeTrue())
			})
		})

		Context("update managed resource", func() {
			var (
				newResource        *corev1.Secret
				newResourceData    []byte
				newResourceDataKey string
			)

			BeforeEach(func() {
				newResource = &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      newResourceName,
						Namespace: namespaceName,
					},
					Data: map[string][]byte{
						"entry": []byte("value"),
					},
					Type: corev1.SecretTypeOpaque,
				}
				var err error
				newResourceData, err = json.Marshal(newResource)
				Expect(err).NotTo(HaveOccurred())
				Expect(newResourceData).NotTo(BeNil())
				newResourceDataKey = fmt.Sprintf("%s.yaml", newResourceName)
			})

			JustBeforeEach(func() {
				Expect(testClient.Create(ctx, secretForManagedResource)).To(Succeed())
				Expect(testClient.Create(ctx, managedResource)).To(Succeed())

				Eventually(func() bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return err == nil && condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should successfully create a new resource when the secret referenced by the managed resource is updated with data containing the new resource", func() {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				updatedSecretForManagedResource := secretForManagedResource.DeepCopy()
				updatedSecretForManagedResource.Data[newResourceDataKey] = newResourceData
				Expect(testClient.Update(ctx, updatedSecretForManagedResource)).To(Succeed())

				Eventually(func() error {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
					if err != nil {
						return err
					}
					return testClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)
				}, time.Minute, 5*time.Second).Should(Succeed())

				Eventually(func() bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return err == nil && condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should successfully update the managed resource with a new secret reference", func() {
				newConfigMapName := "new-configmap"
				newConfigMap := &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: corev1.SchemeGroupVersion.String(),
						Kind:       "ConfigMap",
					}, ObjectMeta: metav1.ObjectMeta{
						Name:      newConfigMapName,
						Namespace: namespaceName,
					}}

				data, err := createSecretDataFromObject(newConfigMap, fmt.Sprintf("%s.yaml", newConfigMapName))
				Expect(err).ToNot(HaveOccurred())
				Expect(data).ToNot(BeNil())

				newSecretForManagedResource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "new-secret",
						Namespace: namespaceName,
					},
					Data: data,
				}

				managedResource.Spec.SecretRefs = append(managedResource.Spec.SecretRefs, corev1.LocalObjectReference{Name: newSecretForManagedResource.Name})
				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

				Expect(testClient.Create(ctx, newSecretForManagedResource)).To(Succeed())
				Expect(testClient.Update(ctx, managedResource)).To(Succeed())

				Eventually(func() error {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
					if err != nil {
						return err
					}
					return testClient.Get(ctx, client.ObjectKeyFromObject(newConfigMap), newConfigMap)
				}, time.Minute, 5*time.Second).Should(Succeed())

				Eventually(func() bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return err == nil && condition != nil && condition.Status == gardencorev1beta1.ConditionTrue
				}, time.Minute, time.Second).Should(BeTrue())
			})

			It("should fail to update the managed resource if a new incorrect resource is added to the secret referenced by the managed resource", func() {
				newResource.TypeMeta = metav1.TypeMeta{}
				newResourceData, err := json.Marshal(newResource)
				Expect(err).NotTo(HaveOccurred())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretForManagedResource), secretForManagedResource)).To(Or(Succeed(), BeNoMatchError()))
				secretForManagedResource.Data[newResourceDataKey] = newResourceData
				Expect(testClient.Update(ctx, secretForManagedResource)).To(Succeed())

				Eventually(func() bool {
					err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)
					condition := gardenerv1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
					return err == nil && condition != nil && condition.Status == gardencorev1beta1.ConditionFalse
				}, time.Minute, time.Second).Should(BeTrue())
			})
		})
	})
})

func createSecretDataFromObject(obj runtime.Object, key string) (map[string][]byte, error) {
	jsonObject, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	data := make(map[string][]byte)
	data[key] = jsonObject
	return data, nil
}
