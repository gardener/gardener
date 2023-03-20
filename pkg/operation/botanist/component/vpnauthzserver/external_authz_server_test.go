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

package vpnauthzserver_test

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpnauthzserver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ExtAuthzServer", func() {
	var (
		ctx context.Context
		c   client.Client

		defaultDepWaiter component.DeployWaiter
		namespace        = "shoot--foo--bar"
		version          *semver.Version

		image             = "some-image"
		maxSurge          = intstr.FromInt(100)
		maxUnavailable    = intstr.FromInt(0)
		maxUnavailablePDB = intstr.FromInt(1)
		vpaUpdateMode     = vpaautoscalingv1.UpdateModeAuto

		deploymentName = "reversed-vpn-auth-server"
		serviceName    = "reversed-vpn-auth-server"
		vpaName        = fmt.Sprintf("%s-vpa", "reversed-vpn-auth-server")

		expectedDeployment                 *appsv1.Deployment
		expectedDestinationRule            *istionetworkingv1beta1.DestinationRule
		expectedService                    *corev1.Service
		expectedVirtualService             *istionetworkingv1beta1.VirtualService
		expectedVpa                        *vpaautoscalingv1.VerticalPodAutoscaler
		expectedPodDisruptionBudgetV1beta1 *policyv1beta1.PodDisruptionBudget
		expectedPodDisruptionBudgetV1      *policyv1.PodDisruptionBudget
	)

	BeforeEach(func() {
		ctx = context.TODO()
		s := runtime.NewScheme()
		Expect(istionetworkingv1beta1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1alpha3.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(appsv1.AddToScheme(s)).To(Succeed())
		Expect(vpaautoscalingv1.AddToScheme(s)).To(Succeed())
		Expect(policyv1beta1.AddToScheme(s)).To(Succeed())
		Expect(policyv1.AddToScheme(s)).To(Succeed())
		Expect(schedulingv1.AddToScheme(s)).To(Succeed())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		var err error
		Expect(err).NotTo(HaveOccurred())

		expectedDeployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
				Labels: map[string]string{
					"app": "reversed-vpn-auth-server",
					"high-availability-config.resources.gardener.cloud/type": "server",
				},
				ResourceVersion: "1",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app": "reversed-vpn-auth-server",
				}},
				Strategy: appsv1.DeploymentStrategy{
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxUnavailable: &maxUnavailable,
						MaxSurge:       &maxSurge,
					},
					Type: appsv1.RollingUpdateDeploymentStrategyType,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "reversed-vpn-auth-server",
						},
					},
					Spec: corev1.PodSpec{
						AutomountServiceAccountToken: pointer.Bool(false),
						PriorityClassName:            v1beta1constants.PriorityClassNameSeedSystem900,
						DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
						Containers: []corev1.Container{
							{
								Name:            "reversed-vpn-auth-server",
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Ports: []corev1.ContainerPort{
									{
										Name:          "grpc-authz",
										ContainerPort: 9001,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("100Mi"),
									},
								},
							},
						},
					},
				},
			},
		}

		expectedDestinationRule = &istionetworkingv1beta1.DestinationRule{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "DestinationRule",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            deploymentName,
				Namespace:       namespace,
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.DestinationRule{
				ExportTo: []string{"*"},
				Host:     fmt.Sprintf("%s.%s.svc.cluster.local", "reversed-vpn-auth-server", namespace),
				TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
					ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
						Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
							MaxConnections: 5000,
							TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
								Interval: &durationpb.Duration{
									Seconds: 75,
								},
								Time: &durationpb.Duration{
									Seconds: 7200,
								},
							},
						},
					},
					LoadBalancer: &istioapinetworkingv1beta1.LoadBalancerSettings{
						LocalityLbSetting: &istioapinetworkingv1beta1.LocalityLoadBalancerSetting{
							Enabled:          &wrapperspb.BoolValue{Value: true},
							FailoverPriority: []string{"topology.kubernetes.io/zone"},
						},
					},
					OutlierDetection: &istioapinetworkingv1beta1.OutlierDetection{
						MinHealthPercent: 0,
					},
					Tls: &istioapinetworkingv1beta1.ClientTLSSettings{
						Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE,
					},
				},
			},
		}

		expectedService = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.istio.io/exportTo":                            "*",
					"networking.resources.gardener.cloud/namespace-selectors": `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchExpressions":[{"key":"handler.exposureclass.gardener.cloud/name","operator":"Exists"}]}]`,
				},
				ResourceVersion: "1",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": deploymentName,
				},
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       "grpc-authz",
						Port:       9001,
						TargetPort: intstr.FromInt(9001),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}

		expectedVirtualService = &istionetworkingv1beta1.VirtualService{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "VirtualService",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            deploymentName,
				Namespace:       namespace,
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.VirtualService{
				ExportTo: []string{"*"},
				Hosts:    []string{fmt.Sprintf("%s.%s.svc.cluster.local", "reversed-vpn-auth-server", namespace)},
				Http: []*istioapinetworkingv1beta1.HTTPRoute{{
					Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{{
						Destination: &istioapinetworkingv1beta1.Destination{
							Host: "reversed-vpn-auth-server",
							Port: &istioapinetworkingv1beta1.PortSelector{Number: 9001},
						},
					}},
				}},
			},
		}

		expectedVpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace, ResourceVersion: "1"},
			TypeMeta:   metav1.TypeMeta{Kind: "VerticalPodAutoscaler", APIVersion: "autoscaling.k8s.io/v1"},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       "reversed-vpn-auth-server",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "reversed-vpn-auth-server",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
						},
					},
				},
			},
		}

		expectedPodDisruptionBudgetV1beta1 = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            deploymentName + "-pdb",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels: map[string]string{
					"app": deploymentName,
				},
			},
			TypeMeta: metav1.TypeMeta{Kind: "PodDisruptionBudget", APIVersion: "policy/v1beta1"},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailablePDB,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": deploymentName,
					},
				},
			},
		}

		expectedPodDisruptionBudgetV1 = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            deploymentName + "-pdb",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels: map[string]string{
					"app": deploymentName,
				},
			},
			TypeMeta: metav1.TypeMeta{Kind: "PodDisruptionBudget", APIVersion: "policy/v1"},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailablePDB,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": deploymentName,
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		defaultDepWaiter = New(c, namespace, image, version)
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actualDeployment := &appsv1.Deployment{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedDeployment.Namespace, expectedDeployment.Name), actualDeployment)).To(Succeed())
			Expect(actualDeployment).To(DeepEqual(expectedDeployment))

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule).To(BeComparableTo(expectedDestinationRule, test.CmpOptsForDestinationRule()))

			actualService := &corev1.Service{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedService.Namespace, expectedService.Name), actualService)).To(Succeed())
			Expect(actualService).To(DeepEqual(expectedService))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), actualVirtualService)).To(Succeed())
			Expect(actualVirtualService).To(BeComparableTo(expectedVirtualService, test.CmpOptsForVirtualService()))

			actualVpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedVpa.Namespace, expectedVpa.Name), actualVpa)).To(Succeed())
			Expect(actualVpa).To(DeepEqual(expectedVpa))
		})

		Context("Kubernetes version >= v1.21", func() {
			BeforeEach(func() {
				version = semver.MustParse("1.22.0")
			})

			It("should succesfully deploy all the components", func() {
				actualPodDisruptionBudget := &policyv1.PodDisruptionBudget{}
				Expect(c.Get(ctx, kubernetesutils.Key(expectedPodDisruptionBudgetV1.Namespace, expectedPodDisruptionBudgetV1.Name), actualPodDisruptionBudget)).To(Succeed())
				Expect(actualPodDisruptionBudget).To(DeepEqual(expectedPodDisruptionBudgetV1))
			})
		})
		Context("Kubernetes version < v1.21", func() {
			BeforeEach(func() {
				version = semver.MustParse("1.20.0")
			})

			It("should succesfully deploy all the components", func() {
				actualPodDisruptionBudget := &policyv1beta1.PodDisruptionBudget{}
				Expect(c.Get(ctx, kubernetesutils.Key(expectedPodDisruptionBudgetV1beta1.Namespace, expectedPodDisruptionBudgetV1beta1.Name), actualPodDisruptionBudget)).To(Succeed())
				Expect(actualPodDisruptionBudget).To(DeepEqual(expectedPodDisruptionBudgetV1beta1))
			})
		})
	})

	Describe("#Destroy", func() {
		JustBeforeEach(func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, kubernetesutils.Key(expectedDeployment.Namespace, expectedDeployment.Name), &appsv1.Deployment{})).To(Succeed())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), &istionetworkingv1beta1.DestinationRule{})).To(Succeed())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedService.Namespace, expectedService.Name), &corev1.Service{})).To(Succeed())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), &istionetworkingv1beta1.VirtualService{})).To(Succeed())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedVpa.Namespace, expectedVpa.Name), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(Succeed())
		})
		AfterEach(func() {
			Expect(c.Get(ctx, kubernetesutils.Key(expectedDeployment.Namespace, expectedDeployment.Name), &appsv1.Deployment{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedService.Namespace, expectedService.Name), &corev1.Service{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kubernetesutils.Key(expectedVpa.Namespace, expectedVpa.Name), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
		})

		Context("Kubernetes version >= v1.21", func() {
			BeforeEach(func() {
				version = semver.MustParse("1.22.0")
			})

			It("should succesfully delete all the components", func() {
				Expect(c.Get(ctx, kubernetesutils.Key(expectedPodDisruptionBudgetV1.Namespace, expectedPodDisruptionBudgetV1.Name), &policyv1.PodDisruptionBudget{})).To(Succeed())
				Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
				Expect(c.Get(ctx, kubernetesutils.Key(expectedPodDisruptionBudgetV1.Namespace, expectedPodDisruptionBudgetV1.Name), &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
			})
		})

		Context("Kubernetes version < v1.21", func() {
			BeforeEach(func() {
				version = semver.MustParse("1.20.0")
			})

			It("should succesfully delete all the components", func() {
				Expect(c.Get(ctx, kubernetesutils.Key(expectedPodDisruptionBudgetV1beta1.Namespace, expectedPodDisruptionBudgetV1beta1.Name), &policyv1beta1.PodDisruptionBudget{})).To(Succeed())
				Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
				Expect(c.Get(ctx, kubernetesutils.Key(expectedPodDisruptionBudgetV1beta1.Namespace, expectedPodDisruptionBudgetV1beta1.Name), &policyv1beta1.PodDisruptionBudget{})).To(BeNotFoundError())
			})
		})
	})

	Describe("#Wait", func() {
		It("should succeed because it's not implemented", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should succeed because it's not implemented", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
