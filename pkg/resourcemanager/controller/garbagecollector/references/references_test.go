// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package references_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
)

var _ = Describe("References", func() {
	var (
		kind = "kind"
		name = "name"
		key  = fmt.Sprintf("reference.resources.gardener.cloud/%s-82a3537f", kind)
	)

	Describe("#AnnotationKey", func() {
		It("should compute the expected key", func() {
			Expect(AnnotationKey(kind, name)).To(Equal(key))
		})
	})

	Describe("#KindFromAnnotationKey", func() {
		It("should return the expected kind", func() {
			Expect(KindFromAnnotationKey(key)).To(Equal(kind))
		})

		It("should return empty string because key doesn't start as expected", func() {
			Expect(KindFromAnnotationKey("foobar/secret/name")).To(BeEmpty())
		})

		It("should return empty string because key doesn't match expected format", func() {
			Expect(KindFromAnnotationKey("reference.resources.gardener.cloud/secret/name/foo")).To(BeEmpty())
		})
	})

	Describe("#InjectAnnotations", func() {
		var (
			configMap1            = "cm1"
			configMap2            = "cm2"
			configMap3            = "cm3"
			configMap4            = "cm4"
			configMap5            = "cm5"
			configMap6            = "cm6"
			secret1               = "secret1"
			secret2               = "secret2"
			secret3               = "secret3"
			secret4               = "secret4"
			secret5               = "secret5"
			secret6               = "secret6"
			secret7               = "secret7"
			additionalAnnotation1 = "foo"
			additionalAnnotation2 = "bar"

			annotations = map[string]string{
				"some-existing": "annotation",
				"reference.resources.gardener.cloud/configmap-1234567": "cm0",
				"reference.resources.gardener.cloud/secret-1234567":    "secret0",
			}
			podSpec = corev1.PodSpec{
				Containers: []corev1.Container{
					{
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMap3,
									},
								},
							},
						},
						Env: []corev1.EnvVar{
							{
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap5,
										},
									},
								},
							},
						},
					},
					{
						EnvFrom: []corev1.EnvFromSource{
							{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secret3,
									},
								},
							},
							{
								SecretRef: &corev1.SecretEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secret4,
									},
								},
							},
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMap4,
									},
								},
							},
						},
						Env: []corev1.EnvVar{
							{
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secret5,
										},
									},
								},
							},
						},
					},
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: secret7},
				},
				Volumes: []corev1.Volume{
					{
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: configMap1},
							},
						},
					},
					{
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: secret1,
							},
						},
					},
					{
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: configMap2},
							},
						},
					},
					{
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: secret2,
							},
						},
					},
					{
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret6,
											},
										},
									},
									{
										ConfigMap: &corev1.ConfigMapProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMap6,
											},
										},
									},
								},
							},
						},
					},
				},
			}
			expectedAnnotationsWithExisting = map[string]string{
				"some-existing":                          "annotation",
				AnnotationKey(KindConfigMap, configMap1): configMap1,
				AnnotationKey(KindConfigMap, configMap2): configMap2,
				AnnotationKey(KindConfigMap, configMap3): configMap3,
				AnnotationKey(KindConfigMap, configMap4): configMap4,
				AnnotationKey(KindConfigMap, configMap5): configMap5,
				AnnotationKey(KindConfigMap, configMap6): configMap6,
				AnnotationKey(KindSecret, secret1):       secret1,
				AnnotationKey(KindSecret, secret2):       secret2,
				AnnotationKey(KindSecret, secret3):       secret3,
				AnnotationKey(KindSecret, secret4):       secret4,
				AnnotationKey(KindSecret, secret5):       secret5,
				AnnotationKey(KindSecret, secret6):       secret6,
				AnnotationKey(KindSecret, secret7):       secret7,
				additionalAnnotation1:                    "",
				additionalAnnotation2:                    "",
			}
			expectedAnnotationsWithoutExisting = map[string]string{
				AnnotationKey(KindConfigMap, configMap1): configMap1,
				AnnotationKey(KindConfigMap, configMap2): configMap2,
				AnnotationKey(KindConfigMap, configMap3): configMap3,
				AnnotationKey(KindConfigMap, configMap4): configMap4,
				AnnotationKey(KindConfigMap, configMap5): configMap5,
				AnnotationKey(KindConfigMap, configMap6): configMap6,
				AnnotationKey(KindSecret, secret1):       secret1,
				AnnotationKey(KindSecret, secret2):       secret2,
				AnnotationKey(KindSecret, secret3):       secret3,
				AnnotationKey(KindSecret, secret4):       secret4,
				AnnotationKey(KindSecret, secret5):       secret5,
				AnnotationKey(KindSecret, secret6):       secret6,
				AnnotationKey(KindSecret, secret7):       secret7,
				additionalAnnotation1:                    "",
				additionalAnnotation2:                    "",
			}

			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: podSpec,
			}
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			deploymentV1beta2 = &appsv1beta2.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1beta2.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			deploymentV1beta1 = &appsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1beta1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			statefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			statefulSetV1beta2 = &appsv1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1beta2.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			statefulSetV1beta1 = &appsv1beta1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1beta1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			daemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			daemonSetV1beta2 = &appsv1beta2.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: appsv1beta2.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			job = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			cronJob = &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
			cronJobV1beta1 = &batchv1beta1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: batchv1beta1.CronJobSpec{
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
		)

		It("should do nothing because object is not handled", func() {
			Expect(InjectAnnotations(&corev1.Service{}, "foo")).To(MatchError(ContainSubstring("unhandled object type")))
		})

		DescribeTable("should behave properly",
			func(obj runtime.Object, matchers ...func()) {
				Expect(InjectAnnotations(obj, additionalAnnotation1, additionalAnnotation2)).To(Succeed())

				for _, matcher := range matchers {
					matcher()
				}
			},

			Entry("corev1.Pod",
				pod,
				func() {
					Expect(pod.Annotations).To(Equal(expectedAnnotationsWithExisting))
				},
			),
			Entry("appsv1.Deployment",
				deployment,
				func() {
					Expect(deployment.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(deployment.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("appsv1beta2.Deployment",
				deploymentV1beta2,
				func() {
					Expect(deploymentV1beta2.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(deploymentV1beta2.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("appsv1beta1.Deployment",
				deploymentV1beta1,
				func() {
					Expect(deploymentV1beta1.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(deploymentV1beta1.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("appsv1.StatefulSet",
				statefulSet,
				func() {
					Expect(statefulSet.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(statefulSet.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("appsv1beta2.StatefulSet",
				statefulSetV1beta2,
				func() {
					Expect(statefulSetV1beta2.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(statefulSetV1beta2.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("appsv1beta1.StatefulSet",
				statefulSetV1beta1,
				func() {
					Expect(statefulSetV1beta1.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(statefulSetV1beta1.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("appsv1.DaemonSet",
				daemonSet,
				func() {
					Expect(daemonSet.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(daemonSet.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("appsv1beta2.DaemonSet",
				daemonSetV1beta2,
				func() {
					Expect(daemonSetV1beta2.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(daemonSetV1beta2.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("batchv1.Job",
				job,
				func() {
					Expect(job.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(job.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("batchv1.CronJob",
				cronJob,
				func() {
					Expect(cronJob.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(cronJob.Spec.JobTemplate.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
					Expect(cronJob.Spec.JobTemplate.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
			Entry("batchv1beta1.CronJob",
				cronJobV1beta1,
				func() {
					Expect(cronJob.Annotations).To(Equal(expectedAnnotationsWithExisting))
					Expect(cronJob.Spec.JobTemplate.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
					Expect(cronJob.Spec.JobTemplate.Spec.Template.Annotations).To(Equal(expectedAnnotationsWithoutExisting))
				},
			),
		)
	})
})
