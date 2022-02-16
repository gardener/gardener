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

package health_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health"
)

var _ = Describe("CheckHealth", func() {
	var (
		ctx context.Context
		c   client.Client

		healthy, unhealthy runtime.Object
		gvk                schema.GroupVersionKind
	)

	BeforeEach(func() {
		ctx = context.Background()
		// client is only needed for fetching events for services
		c = nil
	})

	It("should return an error if GVK cannot be determined", func() {
		// erase unstructured object needs GVK
		obj := &unstructured.Unstructured{}
		checked, err := CheckHealth(ctx, c, obj)
		Expect(checked).To(BeFalse())
		Expect(err).To(MatchError(ContainSubstring("unstructured object has no kind")))
	})

	Context("object type not handled (Namespace)", func() {
		BeforeEach(func() {
			gvk = corev1.SchemeGroupVersion.WithKind("Namespace")

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
			checked, err := CheckHealth(ctx, c, healthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			checked, err = CheckHealth(ctx, c, prepareUnstructured(healthy, gvk))
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			checked, err = CheckHealth(ctx, c, unhealthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			checked, err = CheckHealth(ctx, c, prepareUnstructured(unhealthy, gvk))
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("object type not registered in scheme (Shoot)", func() {
		BeforeEach(func() {
			gvk = gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")

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
			checked, err := CheckHealth(ctx, c, healthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			checked, err = CheckHealth(ctx, c, prepareUnstructured(healthy, gvk))
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			checked, err = CheckHealth(ctx, c, unhealthy)
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			checked, err = CheckHealth(ctx, c, prepareUnstructured(unhealthy, gvk))
			Expect(checked).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	testSuite := func() {
		It("should not return an error for healthy object", Offset(1), func() {
			checked, err := CheckHealth(ctx, c, healthy)
			Expect(checked).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an error for healthy object (unstructured)", Offset(1), func() {
			checked, err := CheckHealth(ctx, c, prepareUnstructured(healthy, gvk))
			Expect(checked).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error for unhealthy object", Offset(1), func() {
			checked, err := CheckHealth(ctx, c, unhealthy)
			Expect(checked).To(BeTrue())
			// we don't care about the particular error here only that it occurred, it is already verified by the respective unit tests
			Expect(err).To(HaveOccurred())
		})

		It("should return an error for unhealthy object (unstructured)", Offset(1), func() {
			checked, err := CheckHealth(ctx, c, prepareUnstructured(unhealthy, gvk))
			Expect(checked).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if conversion to structured type fails", Offset(1), func() {
			obj := prepareUnstructured(unhealthy, gvk)
			// resourceVersion is a string, converting this unstructured object to structured object will fail
			resourceVersion := int64(1234)
			Expect(unstructured.SetNestedField(obj.Object, resourceVersion, "metadata", "resourceVersion")).To(Succeed())

			checked, err := CheckHealth(ctx, c, obj)
			Expect(checked).To(BeFalse())
			Expect(err).To(MatchError(ContainSubstring("unable to convert unstructured object")))
		})
	}

	Context("CustomResourceDefinition", func() {
		Context("apiextensions/v1", func() {
			BeforeEach(func() {
				gvk = apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition")

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
			})

			testSuite()
		})

		Context("apiextensions/v1beta1", func() {
			BeforeEach(func() {
				gvk = apiextensionsv1beta1.SchemeGroupVersion.WithKind("CustomResourceDefinition")

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
			})

			testSuite()
		})
	})

	Context("DaemonSet", func() {
		BeforeEach(func() {
			gvk = appsv1.SchemeGroupVersion.WithKind("DaemonSet")

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
				},
			}
		})

		testSuite()
	})

	Context("Deployment", func() {
		BeforeEach(func() {
			gvk = appsv1.SchemeGroupVersion.WithKind("Deployment")

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
		})

		testSuite()
	})

	Context("Job", func() {
		BeforeEach(func() {
			gvk = batchv1.SchemeGroupVersion.WithKind("Job")

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
		})

		testSuite()
	})

	Context("Pod", func() {
		BeforeEach(func() {
			gvk = corev1.SchemeGroupVersion.WithKind("Pod")

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
		})

		testSuite()
	})

	Context("ReplicaSet", func() {
		BeforeEach(func() {
			gvk = appsv1.SchemeGroupVersion.WithKind("ReplicaSet")

			healthy = &appsv1.ReplicaSet{
				Spec:   appsv1.ReplicaSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.ReplicaSetStatus{ReadyReplicas: 2},
			}
			unhealthy = &appsv1.ReplicaSet{
				Spec:   appsv1.ReplicaSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
			}
		})

		testSuite()
	})

	Context("ReplicationController", func() {
		BeforeEach(func() {
			gvk = corev1.SchemeGroupVersion.WithKind("ReplicationController")

			healthy = &corev1.ReplicationController{
				Spec:   corev1.ReplicationControllerSpec{Replicas: pointer.Int32(2)},
				Status: corev1.ReplicationControllerStatus{ReadyReplicas: 2},
			}
			unhealthy = &corev1.ReplicationController{
				Spec:   corev1.ReplicationControllerSpec{Replicas: pointer.Int32(2)},
				Status: corev1.ReplicationControllerStatus{ReadyReplicas: 1},
			}
		})

		testSuite()
	})

	Context("Service", func() {
		BeforeEach(func() {
			gvk = corev1.SchemeGroupVersion.WithKind("Service")

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

			c = fakeclient.NewClientBuilder().Build()
		})

		testSuite()
	})

	Context("StatefulSet", func() {
		BeforeEach(func() {
			gvk = appsv1.SchemeGroupVersion.WithKind("StatefulSet")

			healthy = &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: pointer.Int32(1)},
				Status: appsv1.StatefulSetStatus{CurrentReplicas: 1, ReadyReplicas: 1},
			}
			unhealthy = &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: pointer.Int32(2)},
				Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
			}
		})

		testSuite()
	})
})

// schemeForPreparation is only supposed to be used for preparing unstructured objects for tests
var schemeForPreparation *runtime.Scheme

func init() {
	schemeForPreparation = runtime.NewScheme()
	utilruntime.Must(kubernetesscheme.AddToScheme(schemeForPreparation))
	utilruntime.Must(gardencorev1beta1.AddToScheme(schemeForPreparation))
	apiextensionsinstall.Install(schemeForPreparation)
}

func prepareUnstructured(obj runtime.Object, expectedGVK schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	Expect(schemeForPreparation.Convert(obj, u, nil)).To(Succeed())

	// make sure our unstructured conversion works as expected, otherwise our test runs with wrongly prepared objects
	Expect(u.GetObjectKind().GroupVersionKind()).To(Equal(expectedGVK), "preparing unstructured object resulted in unexpected GVK")

	return u
}
