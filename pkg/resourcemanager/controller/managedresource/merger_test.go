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

package managedresource

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("merger", func() {

	origin := "test:a/b"

	Describe("#merge", func() {
		var (
			current, desired *unstructured.Unstructured
		)

		BeforeEach(func() {
			oldPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion:            "123",
					Finalizers:                 []string{"finalizer"},
					Name:                       "foo-abcdef",
					Namespace:                  "bar",
					Generation:                 42,
					CreationTimestamp:          metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
					DeletionTimestamp:          &metav1.Time{Time: time.Now().Add(1 * time.Hour)},
					UID:                        "8c3d49f6-e177-4938-8547-c61283a84876",
					GenerateName:               "foo",
					ClusterName:                "shoot",
					DeletionGracePeriodSeconds: pointer.Int64Ptr(30),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "v1",
						Kind:               "Namespace",
						Name:               "default",
						UID:                "18590d53-3e4d-4616-b411-88212dc69ac6",
						Controller:         pointer.BoolPtr(true),
						BlockOwnerDeletion: pointer.BoolPtr(true),
					}},
				},
			}

			oldJSON, err := runtime.DefaultUnstructuredConverter.ToUnstructured(oldPod)
			Expect(err).NotTo(HaveOccurred())
			current = &unstructured.Unstructured{
				Object: oldJSON,
			}

			desired = current.DeepCopy()
		})

		It("should not overwrite current .metadata", func() {
			desired.Object["metadata"] = nil

			expected := current.DeepCopy()
			addAnnotations(origin, expected)

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.Object["metadata"]).To(Equal(expected.Object["metadata"]))
		})

		It("should force overwrite current .metadata.labels", func() {
			current.SetLabels(map[string]string{"foo": "bar"})
			desired.SetLabels(map[string]string{"other": "baz"})
			existingLabels := map[string]string{"existing": "ignored"}

			expected := desired.DeepCopy()

			Expect(merge(origin, desired, current, true, existingLabels, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetLabels()).To(Equal(expected.GetLabels()))
		})

		It("should merge current and desired .metadata.labels", func() {
			current.SetLabels(map[string]string{"foo": "bar"})
			desired.SetLabels(map[string]string{"other": "baz"})
			existingLabels := map[string]string{"existing": "ignored"}

			expected := desired.DeepCopy()
			expected.SetLabels(map[string]string{
				"foo":   "bar",
				"other": "baz",
			})

			Expect(merge(origin, desired, current, false, existingLabels, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetLabels()).To(Equal(expected.GetLabels()))
		})

		It("should remove labels from current .metadata.labels which have been remove from the mr secret", func() {
			current.SetLabels(map[string]string{"foo": "bar"})
			desired.SetLabels(map[string]string{"other": "baz"})
			existingLabels := map[string]string{"foo": "bar"} // foo: bar removed from specification in mr secret

			expected := desired.DeepCopy()
			expected.SetLabels(map[string]string{
				"other": "baz",
			})

			Expect(merge(origin, desired, current, false, existingLabels, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetLabels()).To(Equal(expected.GetLabels()))
		})

		It("should not remove labels from current .metadata.labels which have been remove from the mr secret but were changed", func() {
			current.SetLabels(map[string]string{"foo": "changed"})
			desired.SetLabels(map[string]string{"other": "baz"})
			existingLabels := map[string]string{"foo": "bar"} // foo: bar removed from specification in mr secret

			expected := desired.DeepCopy()
			expected.SetLabels(map[string]string{
				"foo":   "changed",
				"other": "baz",
			})

			Expect(merge(origin, desired, current, false, existingLabels, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetLabels()).To(Equal(expected.GetLabels()))
		})

		It("should force overwrite current .metadata.annotations", func() {
			current.SetAnnotations(map[string]string{"foo": "bar"})
			desired.SetAnnotations(map[string]string{"other": "baz"})
			existingAnnotations := map[string]string{"existing": "ignored"}

			expected := desired.DeepCopy()
			addAnnotations(origin, expected)

			Expect(merge(origin, desired, current, false, nil, true, existingAnnotations, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetAnnotations()).To(Equal(expected.GetAnnotations()))
		})

		It("should merge current and desired .metadata.annotations", func() {
			current.SetAnnotations(map[string]string{"foo": "bar"})
			desired.SetAnnotations(map[string]string{"other": "baz"})
			existingAnnotations := map[string]string{"existing": "ignored"}

			expected := desired.DeepCopy()
			expected.SetAnnotations(map[string]string{
				"foo":   "bar",
				"other": "baz",
			})
			addAnnotations(origin, expected)

			Expect(merge(origin, desired, current, false, nil, false, existingAnnotations, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetAnnotations()).To(Equal(expected.GetAnnotations()))
		})

		It("should remove annotations from current .metadata.annotations which were once specified in the mr secret", func() {
			current.SetAnnotations(map[string]string{"foo": "bar"})
			desired.SetAnnotations(map[string]string{"other": "baz"})
			existingAnnotations := map[string]string{"foo": "bar"} // foo: bar removed from specification in mr secret

			expected := desired.DeepCopy()
			expected.SetAnnotations(map[string]string{
				"other": "baz",
			})
			addAnnotations(origin, expected)

			Expect(merge(origin, desired, current, false, nil, false, existingAnnotations, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetAnnotations()).To(Equal(expected.GetAnnotations()))
		})

		It("should not remove annotations from current .metadata.annotations which have been remove from the mr secret but were changed", func() {
			current.SetAnnotations(map[string]string{"foo": "changed"})
			desired.SetAnnotations(map[string]string{"other": "baz"})
			existingAnnotations := map[string]string{"foo": "bar"} // foo: bar removed from specification in mr secret

			expected := desired.DeepCopy()
			expected.SetAnnotations(map[string]string{
				"foo":   "changed",
				"other": "baz",
			})
			addAnnotations(origin, expected)

			Expect(merge(origin, desired, current, false, nil, false, existingAnnotations, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.GetAnnotations()).To(Equal(expected.GetAnnotations()))
		})

		It("should keep current .status if it is not empty", func() {
			current.Object["status"] = map[string]interface{}{
				"podIP": "1.1.1.1",
			}
			desired.Object["status"] = map[string]interface{}{
				"podIP": "2.2.2.2",
			}

			expected := current.DeepCopy()

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.Object["status"]).To(Equal(expected.Object["status"]))
		})

		It("should discard .status if current .status is empty", func() {
			desired.Object["status"] = map[string]interface{}{
				"podIP": "2.2.2.2",
			}

			current.Object["status"] = map[string]interface{}{}

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.Object["status"]).To(BeNil())
		})

		It("should discard .status if current .status is not set", func() {
			desired.Object["status"] = map[string]interface{}{
				"podIP": "2.2.2.2",
			}

			delete(current.Object, "status")

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(current.Object["status"]).To(BeNil())
		})

		Describe("sets warning annotation", func() {
			AfterEach(func() {
				Expect(current.GetAnnotations()).
					To(HaveKeyWithValue("resources.gardener.cloud/description", descriptionAnnotationText))
			})

			It("when forceOverrideAnnotation is false", func() {
				Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).ToNot(HaveOccurred(), "merge succeeds")
			})
			It("when forceOverrideAnnotation is false and old annotations exist", func() {
				desired.SetAnnotations(map[string]string{"goo": "boo"})
				current.SetAnnotations(map[string]string{"foo": "bar"})
				Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).ToNot(HaveOccurred(), "merge succeeds")

				Expect(current.GetAnnotations()).To(HaveKeyWithValue("goo", "boo"))
				Expect(current.GetAnnotations()).To(HaveKeyWithValue("foo", "bar"))
			})

			It("when forceOverrideAnnotation is true", func() {
				desired.SetAnnotations(map[string]string{"goo": "boo"})
				Expect(merge(origin, desired, current, false, nil, true, nil, false, false)).ToNot(HaveOccurred(), "merge succeeds")
				Expect(current.GetAnnotations()).To(HaveKeyWithValue("goo", "boo"))
			})
		})
	})

	Describe("#mergeDeployment", func() {
		var (
			old, new *appsv1.Deployment
			s        *runtime.Scheme
		)

		BeforeEach(func() {
			s = runtime.NewScheme()
			Expect(appsv1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")

			old = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"controller-uid": "1a2b3c"},
					},
					Replicas: pointer.Int32Ptr(1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "foo-container",
								},
							},
						},
					},
				},
			}

			new = old.DeepCopy()
		})

		It("should not overwrite old .spec.replicas if the new one is nil", func() {
			new.Spec.Replicas = nil

			expected := old.DeepCopy()

			Expect(mergeDeployment(s, old, new, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should not overwrite old .spec.replicas if preserveReplicas is true", func() {
			new.Spec.Replicas = pointer.Int32Ptr(2)

			expected := old.DeepCopy()

			Expect(mergeDeployment(s, old, new, true, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should use new .spec.replicas if preserveReplicas is false", func() {
			new.Spec.Replicas = pointer.Int32Ptr(2)

			expected := new.DeepCopy()

			Expect(mergeDeployment(s, old, new, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})
	})

	Describe("#mergeDeploymentAnnotations", func() {
		origin := "test:a/b"
		var (
			old, new, expected *appsv1.Deployment
			s                  *runtime.Scheme
			current, desired   = &unstructured.Unstructured{}, &unstructured.Unstructured{}
		)

		BeforeEach(func() {
			s = runtime.NewScheme()
			Expect(appsv1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")

			old = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"controller-uid": "1a2b3c"},
					},
					Replicas: pointer.Int32Ptr(1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "foo-container",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("50m"),
											corev1.ResourceMemory: resource.MustParse("150Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("500m"),
											corev1.ResourceMemory: resource.MustParse("1Gi"),
										},
									},
								},
							},
						},
					},
				},
			}

			new = old.DeepCopy()
			expected = old.DeepCopy()
		})

		It("should use new .spec.replicas if preserve-replicas is unset", func() {
			new.Spec.Replicas = pointer.Int32Ptr(2)

			Expect(s.Convert(old, current, nil)).Should(Succeed())
			Expect(s.Convert(new, desired, nil)).Should(Succeed())

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).To(Succeed(), "merge should be successful")
			Expect(s.Convert(current, expected, nil)).Should(Succeed())

			Expect(expected.Spec.Replicas).To(Equal(new.Spec.Replicas))
		})

		It("should not overwrite old .spec.replicas if preserve-replicas is true", func() {
			new.Spec.Replicas = pointer.Int32Ptr(2)
			new.ObjectMeta.Annotations["resources.gardener.cloud/preserve-replicas"] = "true"

			Expect(s.Convert(old, current, nil)).Should(Succeed())
			Expect(s.Convert(new, desired, nil)).Should(Succeed())

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).To(Succeed(), "merge should be successful")
			Expect(s.Convert(current, expected, nil)).Should(Succeed())
			Expect(expected.Spec.Replicas).To(Equal(old.Spec.Replicas))
		})

		It("should use new .spec.template.spec.resources if preserve-resources is unset", func() {
			new.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("60m"),
					corev1.ResourceMemory: resource.MustParse("180Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("600m"),
					corev1.ResourceMemory: resource.MustParse("1.2Gi"),
				},
			}

			Expect(s.Convert(old, current, nil)).Should(Succeed())
			Expect(s.Convert(new, desired, nil)).Should(Succeed())

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).To(Succeed(), "merge should be successful")
			Expect(s.Convert(current, expected, nil)).Should(Succeed())

			Expect(new.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"].Equal(expected.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"])).To(BeTrue())
			Expect(new.Spec.Template.Spec.Containers[0].Resources.Requests["memory"].Equal(expected.Spec.Template.Spec.Containers[0].Resources.Requests["memory"])).To(BeTrue())
			Expect(new.Spec.Template.Spec.Containers[0].Resources.Limits["cpu"].Equal(expected.Spec.Template.Spec.Containers[0].Resources.Limits["cpu"])).To(BeTrue())
			Expect(new.Spec.Template.Spec.Containers[0].Resources.Limits["memory"].Equal(expected.Spec.Template.Spec.Containers[0].Resources.Limits["memory"])).To(BeTrue())
		})

		It("should not overwrite .spec.template.spec.resources if preserve-resources is true", func() {
			new.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("60m"),
					corev1.ResourceMemory: resource.MustParse("180Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("600m"),
					corev1.ResourceMemory: resource.MustParse("1.2Gi"),
				},
			}

			new.ObjectMeta.Annotations["resources.gardener.cloud/preserve-resources"] = "true"

			Expect(s.Convert(old, current, nil)).Should(Succeed())
			Expect(s.Convert(new, desired, nil)).Should(Succeed())

			Expect(merge(origin, desired, current, false, nil, false, nil, false, false)).To(Succeed(), "merge should be successful")
			Expect(s.Convert(current, expected, nil)).Should(Succeed())
			Expect(expected.Spec.Template.Spec.Containers[0].Resources).To(Equal(old.Spec.Template.Spec.Containers[0].Resources))
		})
	})

	Describe("#mergeStatefulset", func() {
		var (
			old, new *appsv1.StatefulSet
			s        *runtime.Scheme
		)

		BeforeEach(func() {
			s = runtime.NewScheme()
			Expect(appsv1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")

			old = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"controller-uid": "1a2b3c"},
					},
					Replicas: pointer.Int32Ptr(1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "foo-container",
								},
							},
						},
					},
				},
			}

			new = old.DeepCopy()
		})

		It("should not overwrite old .spec.replicas if the new one is nil", func() {
			new.Spec.Replicas = nil

			expected := old.DeepCopy()

			Expect(mergeStatefulSet(s, old, new, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should not overwrite old .spec.replicas if preserveReplicas is true", func() {
			new.Spec.Replicas = pointer.Int32Ptr(2)

			expected := old.DeepCopy()

			Expect(mergeStatefulSet(s, old, new, true, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should use new .spec.replicas if preserveReplicas is false", func() {
			new.Spec.Replicas = pointer.Int32Ptr(2)

			expected := new.DeepCopy()

			Expect(mergeStatefulSet(s, old, new, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should use new .spec.replicas if preserveReplicas is false", func() {
			new.Spec.Replicas = pointer.Int32Ptr(2)

			expected := new.DeepCopy()

			Expect(mergeStatefulSet(s, old, new, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should use new .spec.volumeClaimTemplates if the StatefulSet has not been created yet", func() {
			new.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
				{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						VolumeName:       "pvc-foo",
						StorageClassName: pointer.StringPtr("ultra-fast"),
					},
				},
			}

			expected := new.DeepCopy()

			Expect(mergeStatefulSet(s, old, new, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should not overwrite old .spec.volumeClaimTemplates if the StatefulSet has already been created", func() {
			old.CreationTimestamp = metav1.Time{Time: time.Now()}
			new.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
				{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						VolumeName:       "pvc-foo",
						StorageClassName: pointer.StringPtr("ultra-fast"),
					},
				},
			}

			expected := new.DeepCopy()
			expected.Spec.VolumeClaimTemplates = nil

			Expect(mergeStatefulSet(s, old, new, false, false)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})
	})

	Describe("#mergeContainer", func() {
		var (
			old, new *corev1.Container

			quantity1, quantity2 resource.Quantity

			resourceListBothSet, resourceListBothSet2 corev1.ResourceList
		)

		BeforeEach(func() {
			old = &corev1.Container{
				Name: "foo-container",
				Resources: corev1.ResourceRequirements{
					Limits:   nil,
					Requests: nil,
				},
			}

			new = old.DeepCopy()

			q, err := resource.ParseQuantity("200")
			Expect(err).NotTo(HaveOccurred())
			quantity1 = q

			resourceListBothSet = corev1.ResourceList{
				corev1.ResourceCPU:    quantity1,
				corev1.ResourceMemory: quantity1,
			}

			q, err = resource.ParseQuantity("500")
			Expect(err).NotTo(HaveOccurred())
			quantity2 = q

			resourceListBothSet2 = corev1.ResourceList{
				corev1.ResourceCPU:    quantity2,
				corev1.ResourceMemory: quantity2,
			}
		})

		It("should do nothing if resource requirements are not set", func() {
			expected := old.DeepCopy()

			mergeContainer(old, new, true)
			Expect(new).To(Equal(expected))
		})

		It("should use new requests if preserveResources is false)", func() {
			old.Resources.Requests = resourceListBothSet
			new.Resources.Requests = resourceListBothSet2

			expected := new.DeepCopy()

			mergeContainer(old, new, false)
			Expect(new).To(Equal(expected))
		})

		It("should use new limits if preserveResources is false)", func() {
			old.Resources.Limits = resourceListBothSet
			new.Resources.Limits = resourceListBothSet2

			expected := new.DeepCopy()

			mergeContainer(old, new, false)
			Expect(new).To(Equal(expected))
		})

		It("should not overwrite requests if preserveResources is true)", func() {
			new.Resources.Requests = resourceListBothSet
			old.Resources.Requests = resourceListBothSet2

			expected := old.DeepCopy()

			mergeContainer(old, new, true)
			Expect(new).To(Equal(expected))
		})

		It("should not overwrite limits if preserveResources is true)", func() {
			new.Resources.Requests = resourceListBothSet
			old.Resources.Requests = resourceListBothSet2

			expected := old.DeepCopy()

			mergeContainer(old, new, true)
			Expect(new).To(Equal(expected))
		})
	})

	Describe("#mergeJob", func() {
		var (
			old, new *batchv1.Job
			s        *runtime.Scheme
		)

		BeforeEach(func() {
			s = runtime.NewScheme()
			Expect(batchv1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")

			old = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: batchv1.JobSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"controller-uid": "1a2b3c"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"controller-uid": "1a2b3c", "job-name": "pi"},
						},
					},
				},
			}

			new = old.DeepCopy()
		})

		It("should not overwrite old .spec.selector if the new one is nil", func() {
			new.Spec.Selector = nil

			expected := old.DeepCopy()

			Expect(mergeJob(s, old, new)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should not overwrite old .spec.template.labels if the new one is nil", func() {
			new.Spec.Template.Labels = nil

			expected := old.DeepCopy()

			Expect(mergeJob(s, old, new)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})

		It("should be able to merge new .spec.template.labels with the old ones", func() {
			new.Spec.Template.Labels = map[string]string{"app": "myapp", "version": "v0.1.0"}

			expected := old.DeepCopy()
			expected.Spec.Template.Labels = map[string]string{
				"app":            "myapp",
				"controller-uid": "1a2b3c",
				"job-name":       "pi",
				"version":        "v0.1.0",
			}

			Expect(mergeJob(s, old, new)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		})
	})

	Describe("#mergeService", func() {
		var (
			old, new, expected *corev1.Service
			s                  *runtime.Scheme
		)

		BeforeEach(func() {
			s = runtime.NewScheme()
			Expect(corev1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")

			old = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Ports: []corev1.ServicePort{
						{
							Name:       "foo",
							Protocol:   corev1.ProtocolTCP,
							Port:       123,
							TargetPort: intstr.FromInt(919),
						},
					},
					Type:            corev1.ServiceTypeClusterIP,
					SessionAffinity: corev1.ServiceAffinityNone,
					Selector:        map[string]string{"foo": "bar"},
				},
			}

			new = old.DeepCopy()
			expected = old.DeepCopy()
		})

		DescribeTable("ClusterIP to", func(mutator func()) {
			mutator()
			Expect(mergeService(s, old, new)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		},
			Entry("ClusterIP with changed ports", func() {
				new.Spec.Ports[0].Port = 1234
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].TargetPort = intstr.FromInt(989)

				expected = new.DeepCopy()
				new.Spec.ClusterIP = ""
			}),
			Entry("ClusterIP with changed ClusterIP, should not update it", func() {
				new.Spec.ClusterIP = "5.6.7.8"
			}),
			Entry("Headless ClusterIP", func() {
				new.Spec.ClusterIP = "None"
				expected.Spec.ClusterIP = "None"
			}),
			Entry("ClusterIP without passing any type", func() {
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)

				expected = new.DeepCopy()
				new.Spec.ClusterIP = "5.6.7.8"
				new.Spec.Type = ""
			}),
			Entry("NodePort with changed ports", func() {
				new.Spec.Type = corev1.ServiceTypeNodePort
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 444

				expected = new.DeepCopy()
			}),

			Entry("ExternalName removes ClusterIP", func() {
				new.Spec.Type = corev1.ServiceTypeExternalName
				new.Spec.Selector = nil
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 0
				new.Spec.ClusterIP = ""
				new.Spec.ExternalName = "foo.com"
				new.Spec.HealthCheckNodePort = 0

				expected = new.DeepCopy()
			}),
		)

		DescribeTable("NodePort to",
			func(mutator func()) {
				old.Spec.Ports[0].NodePort = 3333
				old.Spec.Type = corev1.ServiceTypeNodePort

				new = old.DeepCopy()
				expected = old.DeepCopy()

				mutator()

				Expect(mergeService(s, old, new)).NotTo(HaveOccurred(), "merge should be successful")
				Expect(new).To(Equal(expected))
			},
			Entry("ClusterIP with changed ports", func() {
				new.Spec.Type = corev1.ServiceTypeClusterIP
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 0

				expected = new.DeepCopy()
			}),
			Entry("ClusterIP with changed ClusterIP", func() {
				new.Spec.ClusterIP = "5.6.7.8"
			}),
			Entry("Headless ClusterIP type service", func() {
				new.Spec.Type = corev1.ServiceTypeClusterIP
				new.Spec.ClusterIP = "None"

				expected = new.DeepCopy()
			}),

			Entry("NodePort with changed ports", func() {
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 444

				expected = new.DeepCopy()
			}),
			Entry("NodePort with changed ports and without nodePort", func() {
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)

				expected = new.DeepCopy()
				new.Spec.Ports[0].NodePort = 0
			}),
			Entry("ExternalName removes ClusterIP", func() {
				new.Spec.Type = corev1.ServiceTypeExternalName
				new.Spec.Selector = nil
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 0
				new.Spec.ClusterIP = ""
				new.Spec.ExternalName = "foo.com"
				new.Spec.HealthCheckNodePort = 0

				expected = new.DeepCopy()
			}),
		)

		DescribeTable("LoadBalancer to", func(mutator func()) {
			old.Spec.Ports[0].NodePort = 3333
			old.Spec.Type = corev1.ServiceTypeLoadBalancer

			new = old.DeepCopy()
			expected = old.DeepCopy()

			mutator()

			Expect(mergeService(s, old, new)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		},
			Entry("ClusterIP with changed ports", func() {
				new.Spec.Type = corev1.ServiceTypeClusterIP
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 0

				expected = new.DeepCopy()
			}),
			Entry("Cluster with ClusterIP changed", func() {
				new.Spec.ClusterIP = "5.6.7.8"
			}),
			Entry("Headless ClusterIP type service", func() {
				new.Spec.Type = corev1.ServiceTypeClusterIP
				new.Spec.ClusterIP = "None"

				expected = new.DeepCopy()
			}),
			Entry("NodePort with changed ports", func() {
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 444

				expected = new.DeepCopy()
			}),
			Entry("NodePort with changed ports and without nodePort", func() {
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)

				expected = new.DeepCopy()
				new.Spec.Ports[0].NodePort = 0
			}),
			Entry("ExternalName removes ClusterIP", func() {
				new.Spec.Type = corev1.ServiceTypeExternalName
				new.Spec.Selector = nil
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 0
				new.Spec.ClusterIP = ""
				new.Spec.ExternalName = "foo.com"
				new.Spec.HealthCheckNodePort = 0

				expected = new.DeepCopy()
			}),
			Entry("LoadBalancer with ExternalTrafficPolicy=Local and HealthCheckNodePort", func() {
				new.Spec.HealthCheckNodePort = 123
				new.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal

				expected = new.DeepCopy()
			}),
			Entry("LoadBalancer with ExternalTrafficPolicy=Local and no HealthCheckNodePort", func() {
				old.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
				old.Spec.HealthCheckNodePort = 3333

				new.Spec.HealthCheckNodePort = 0
				new.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal

				expected = old.DeepCopy()
			}),
			Entry("LoadBalancer should retain spec.loadBalancerIP", func() {
				old.Spec.LoadBalancerIP = "1.2.3.4"

				expected = old.DeepCopy()
			}),
		)

		DescribeTable("ExternalName to", func(mutator func()) {
			old.Spec.Ports[0].NodePort = 0
			old.Spec.Type = corev1.ServiceTypeExternalName
			old.Spec.HealthCheckNodePort = 0
			old.Spec.ClusterIP = ""
			old.Spec.ExternalName = "baz.bar"
			old.Spec.Selector = nil

			new = old.DeepCopy()
			expected = old.DeepCopy()

			mutator()

			Expect(mergeService(s, old, new)).NotTo(HaveOccurred(), "merge should be successful")
			Expect(new).To(Equal(expected))
		},
			Entry("ClusterIP with changed ports", func() {
				new.Spec.Type = corev1.ServiceTypeClusterIP
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 0
				new.Spec.ExternalName = ""
				new.Spec.ClusterIP = "3.4.5.6"

				expected = new.DeepCopy()
			}),
			Entry("NodePort with changed ports", func() {
				new.Spec.Type = corev1.ServiceTypeNodePort
				new.Spec.Ports[0].Protocol = corev1.ProtocolUDP
				new.Spec.Ports[0].Port = 999
				new.Spec.Ports[0].TargetPort = intstr.FromInt(888)
				new.Spec.Ports[0].NodePort = 444
				new.Spec.ExternalName = ""
				new.Spec.ClusterIP = "3.4.5.6"

				expected = new.DeepCopy()
			}),
			Entry("LoadBalancer with ExternalTrafficPolicy=Local and HealthCheckNodePort", func() {
				new.Spec.Type = corev1.ServiceTypeLoadBalancer
				new.Spec.HealthCheckNodePort = 123
				new.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
				new.Spec.ExternalName = ""
				new.Spec.ClusterIP = "3.4.5.6"

				expected = new.DeepCopy()
			}),
		)
	})
})

func addAnnotations(origin string, obj *unstructured.Unstructured) {
	ann := obj.GetAnnotations()

	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[descriptionAnnotation] = descriptionAnnotationText
	ann[originAnnotation] = origin
	obj.SetAnnotations(ann)
}
