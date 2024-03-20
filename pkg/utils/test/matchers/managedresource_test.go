// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package matchers_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/etcd-druid/pkg/client/kubernetes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	testruntime "github.com/gardener/gardener/pkg/utils/test/runtime"
)

var _ = Describe("ManagedResource Object Matcher", func() {
	var (
		fakeClient     client.Client
		containsObject func(client.Object) types.GomegaMatcher
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.Scheme).Build()
		containsObject = NewManagedResourceObjectMatcher(fakeClient)
	})

	Context("without managed resource", func() {
		It("should not find an object", func() {
			Expect(nil).NotTo(containsObject(&corev1.Secret{}))
		})
	})

	Context("with managed resource", func() {
		var (
			namespace       string
			managedResource *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			namespace = "test-namespace"
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
				},
			}
		})

		Context("with secrets references", func() {
			var (
				ctx context.Context

				configMap                                      *corev1.ConfigMap
				deployment                                     *appsv1.Deployment
				managedResourceSecret1, managedResourceSecret2 *corev1.Secret
			)

			BeforeEach(func() {
				ctx = context.Background()

				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "controller-config",
						Namespace: namespace,
					},
					Data: map[string]string{
						"key": "value",
					},
				}
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "admission",
						Namespace: "gardener",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: ptr.To(int32(2)),
						Paused:   true,
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "gardener"},
								},
							},
						},
					},
				}

				managedResourceSecret1 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						fmt.Sprintf("configmap__%s__%s.yaml", configMap.Namespace, configMap.Name): []byte(testruntime.Serialize(configMap, fakeClient.Scheme())),
					},
				}
				managedResourceSecret2 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret2",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						fmt.Sprintf("deployment__%s__%s.yaml", deployment.Namespace, deployment.Name): []byte(testruntime.Serialize(deployment, fakeClient.Scheme())),
					},
				}
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{
					{Name: managedResourceSecret1.Name},
					{Name: managedResourceSecret2.Name},
				}

				Expect(fakeClient.Create(ctx, managedResourceSecret1)).To(Succeed())
				Expect(fakeClient.Create(ctx, managedResourceSecret2)).To(Succeed())
			})

			It("should only find contained resources", func() {
				Expect(managedResource).To(containsObject(configMap))
				Expect(managedResource).To(containsObject(deployment))

				deploymentModified := deployment.DeepCopy()
				deploymentModified.Spec.MinReadySeconds += 1
				Expect(managedResource).NotTo(containsObject(deploymentModified))
			})

			It("should not find resources if data key does not match", func() {
				objectKey := fmt.Sprintf("deployment__%s__%s.yaml", deployment.Namespace, deployment.Name)
				objectData := managedResourceSecret2.Data[objectKey]
				Expect(objectData).NotTo(BeEmpty())

				patch := client.MergeFrom(managedResourceSecret2.DeepCopy())
				delete(managedResourceSecret2.Data, objectKey)
				managedResourceSecret2.Data[strings.Trim(objectKey, ".yaml")] = objectData
				Expect(fakeClient.Patch(ctx, managedResourceSecret2, patch)).To(Succeed())

				Expect(managedResource).NotTo(containsObject(deployment))
			})
		})

		Context("without secret references", func() {
			It("should not find an object", func() {
				Expect(managedResource).NotTo(containsObject(&corev1.Secret{}))
			})
		})
	})

})
