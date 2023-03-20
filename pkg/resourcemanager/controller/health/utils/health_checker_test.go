// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
)

var _ = Describe("CheckHealth", func() {
	var (
		healthy, unhealthy, unhealthyWithSkipHealthCheckAnnotation client.Object
	)

	Context("object type not handled (Namespace)", func() {
		BeforeEach(func() {
			healthy = &corev1.Namespace{
				Status: corev1.NamespaceStatus{
					Phase: corev1.NamespaceActive,
				},
			}
			unhealthy = &corev1.Namespace{
				Status: corev1.NamespaceStatus{
					Phase: corev1.NamespaceTerminating,
				},
			}
		})

		It("should not return an error", func() {
			checked, err := CheckHealth(healthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			checked, err = CheckHealth(unhealthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("object type not registered in scheme (Shoot)", func() {
		BeforeEach(func() {
			healthy = &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Conditions: []gardencorev1beta1.Condition{{
						Type:   gardencorev1beta1.ShootAPIServerAvailable,
						Status: gardencorev1beta1.ConditionTrue,
					}},
				},
			}
			unhealthy = &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Conditions: []gardencorev1beta1.Condition{{
						Type:   gardencorev1beta1.ShootAPIServerAvailable,
						Status: gardencorev1beta1.ConditionFalse,
					}},
				},
			}
		})

		It("should not return an error for unregistered types", func() {
			checked, err := CheckHealth(healthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			checked, err = CheckHealth(unhealthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	testSuite := func() {
		It("should not return an error for healthy object", Offset(1), func() {
			checked, err := CheckHealth(healthy)
			Expect(checked).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error for unhealthy object", Offset(1), func() {
			checked, err := CheckHealth(unhealthy)
			Expect(checked).To(BeTrue())
			// we don't care about the particular error here only that it occurred, it is already verified by the respective unit tests
			Expect(err).To(HaveOccurred())
		})

		It("should not return an error for unhealthy object if it has skip-health-check annotation", Offset(1), func() {
			checked, err := CheckHealth(unhealthyWithSkipHealthCheckAnnotation)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	}

	Context("CustomResourceDefinition", func() {
		Context("apiextensions/v1", func() {
			BeforeEach(func() {
				healthy = &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.NamesAccepted,
								Status: apiextensionsv1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				}
				unhealthy = &apiextensionsv1.CustomResourceDefinition{
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.NamesAccepted,
								Status: apiextensionsv1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1.Terminating,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				}
				unhealthyWithSkipHealthCheckAnnotation = &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							resourcesv1alpha1.SkipHealthCheck: "true",
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.NamesAccepted,
								Status: apiextensionsv1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1.Terminating,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				}
			})

			testSuite()
		})

		Context("apiextensions/v1beta1", func() {
			BeforeEach(func() {
				healthy = &apiextensionsv1beta1.CustomResourceDefinition{
					Status: apiextensionsv1beta1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1beta1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1beta1.NamesAccepted,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1beta1.Established,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
						},
					},
				}
				unhealthy = &apiextensionsv1beta1.CustomResourceDefinition{
					Status: apiextensionsv1beta1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1beta1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1beta1.NamesAccepted,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1beta1.Established,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1beta1.Terminating,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
						},
					},
				}
				unhealthyWithSkipHealthCheckAnnotation = &apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							resourcesv1alpha1.SkipHealthCheck: "true",
						},
					},
					Status: apiextensionsv1beta1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1beta1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1beta1.NamesAccepted,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1beta1.Established,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
							{
								Type:   apiextensionsv1beta1.Terminating,
								Status: apiextensionsv1beta1.ConditionTrue,
							},
						},
					},
				}
			})

			testSuite()
		})
	})

	Context("DaemonSet", func() {
		BeforeEach(func() {
			healthy = &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 1,
					CurrentNumberScheduled: 1,
					NumberReady:            1,
				},
			}
			unhealthy = &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 1,
					CurrentNumberScheduled: 1,
					NumberUnavailable:      1,
				},
			}
			unhealthyWithSkipHealthCheckAnnotation = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 1,
					CurrentNumberScheduled: 1,
					NumberUnavailable:      1,
				},
			}
		})

		testSuite()
	})

	Context("Deployment", func() {
		BeforeEach(func() {
			healthy = &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				}}},
			}
			unhealthy = &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				}}},
			}
			unhealthyWithSkipHealthCheckAnnotation = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				}}},
			}
		})

		testSuite()
	})

	Context("Job", func() {
		BeforeEach(func() {
			healthy = &batchv1.Job{Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionFalse,
				}},
			}}
			unhealthy = &batchv1.Job{Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				}},
			}}
			unhealthyWithSkipHealthCheckAnnotation = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{{
						Type:   batchv1.JobFailed,
						Status: corev1.ConditionTrue,
					}},
				}}
		})

		testSuite()
	})

	Context("Pod", func() {
		BeforeEach(func() {
			healthy = &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}
			unhealthy = &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			}
			unhealthyWithSkipHealthCheckAnnotation = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			}
		})

		testSuite()
	})

	Context("ReplicaSet", func() {
		BeforeEach(func() {
			healthy = &appsv1.ReplicaSet{
				Spec:   appsv1.ReplicaSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.ReplicaSetStatus{ReadyReplicas: 2},
			}
			unhealthy = &appsv1.ReplicaSet{
				Spec:   appsv1.ReplicaSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
			}
			unhealthyWithSkipHealthCheckAnnotation = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				Spec:   appsv1.ReplicaSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
			}
		})

		testSuite()
	})

	Context("ReplicationController", func() {
		BeforeEach(func() {
			healthy = &corev1.ReplicationController{
				Spec:   corev1.ReplicationControllerSpec{Replicas: pointer.Int32(2)},
				Status: corev1.ReplicationControllerStatus{ReadyReplicas: 2},
			}
			unhealthy = &corev1.ReplicationController{
				Spec:   corev1.ReplicationControllerSpec{Replicas: pointer.Int32(2)},
				Status: corev1.ReplicationControllerStatus{ReadyReplicas: 1},
			}
			unhealthyWithSkipHealthCheckAnnotation = &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				Spec:   corev1.ReplicationControllerSpec{Replicas: pointer.Int32(2)},
				Status: corev1.ReplicationControllerStatus{ReadyReplicas: 1},
			}
		})

		testSuite()
	})

	Context("Service", func() {
		BeforeEach(func() {
			healthy = &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{Hostname: "foo.bar"},
						},
					},
				},
			}
			unhealthy = &corev1.Service{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				Spec:     corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			}
			unhealthyWithSkipHealthCheckAnnotation = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				Spec:     corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			}
		})

		testSuite()
	})

	Context("StatefulSet", func() {
		BeforeEach(func() {
			healthy = &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: pointer.Int32(1)},
				Status: appsv1.StatefulSetStatus{CurrentReplicas: 1, ReadyReplicas: 1},
			}
			unhealthy = &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
			}
			unhealthyWithSkipHealthCheckAnnotation = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						resourcesv1alpha1.SkipHealthCheck: "true",
					},
				},
				Spec:   appsv1.StatefulSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
			}
		})

		testSuite()
	})
})
