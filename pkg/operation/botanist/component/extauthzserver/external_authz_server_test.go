// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extauthzserver

import (
	"context"
	"fmt"

	protobuftypes "github.com/gogo/protobuf/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
)

var _ = Describe("ExtAuthzServer", func() {
	var (
		ctx context.Context
		c   client.Client

		defaultDepWaiter component.DeployWaiter
		namespace        = "shoot--foo--bar"

		image                      = "eu.gcr.io/gardener-project/gardener/ext-authz-server:0.1.0"
		replicas             int32 = 1
		revisionHistoryLimit int32 = 1
		maxSurge                   = intstr.FromInt(100)
		maxUnavailable             = intstr.FromInt(0)
		maxUnavailablePDB          = intstr.FromInt(1)
		vpaUpdateMode              = autoscalingv1beta2.UpdateModeAuto

		deploymentName = DeploymentName
		serviceName    = DeploymentName
		vpaName        = fmt.Sprintf("%s-vpa", DeploymentName)

		expectedDeployment          *appsv1.Deployment
		expectedDestinationRule     *istionetworkingv1beta1.DestinationRule
		expectedService             *corev1.Service
		expectedVirtualService      *istionetworkingv1beta1.VirtualService
		expectedVpa                 *autoscalingv1beta2.VerticalPodAutoscaler
		expectedPodDisruptionBudget *policyv1beta1.PodDisruptionBudget
	)

	BeforeEach(func() {
		ctx = context.TODO()
		s := runtime.NewScheme()
		Expect(istionetworkingv1beta1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1alpha3.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(appsv1.AddToScheme(s)).To(Succeed())
		Expect(autoscalingv1beta2.AddToScheme(s)).To(Succeed())
		Expect(policyv1beta1.AddToScheme(s)).To(Succeed())

		c = fake.NewClientBuilder().WithScheme(s).Build()

		var err error
		Expect(err).NotTo(HaveOccurred())

		expectedDeployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
				Labels: map[string]string{
					"app": DeploymentName,
				},
				ResourceVersion: "1",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             &replicas,
				RevisionHistoryLimit: &revisionHistoryLimit,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					v1beta1constants.LabelApp: DeploymentName,
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
							v1beta1constants.LabelApp: DeploymentName,
						},
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: "In",
													Values:   []string{DeploymentName},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						AutomountServiceAccountToken: pointer.Bool(false),
						PriorityClassName:            "system-cluster-critical",
						DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
						Containers: []corev1.Container{
							{
								Name:            DeploymentName,
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
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("500Mi"),
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
				Host:     fmt.Sprintf("%s.%s.svc.cluster.local", DeploymentName, namespace),
				TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
					ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
						Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
							MaxConnections: 5000,
							TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
								Interval: &protobuftypes.Duration{
									Seconds: 75,
								},
								Time: &protobuftypes.Duration{
									Seconds: 7200,
								},
							},
						},
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
					"networking.istio.io/exportTo": "*",
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
						Port:       AuthServerPort,
						TargetPort: intstr.FromInt(AuthServerPort),
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
				Hosts:    []string{fmt.Sprintf("%s.%s.svc.cluster.local", DeploymentName, namespace)},
				Http: []*istioapinetworkingv1beta1.HTTPRoute{{
					Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{{
						Destination: &istioapinetworkingv1beta1.Destination{
							Host: DeploymentName,
							Port: &istioapinetworkingv1beta1.PortSelector{Number: AuthServerPort},
						},
					}},
				}},
			},
		}

		expectedVpa = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace, ResourceVersion: "1"},
			TypeMeta:   metav1.TypeMeta{Kind: "VerticalPodAutoscaler", APIVersion: "autoscaling.k8s.io/v1beta2"},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       DeploymentName,
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
					ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
						{
							ContainerName: DeploymentName,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
						},
					},
				},
			},
		}
	})

	expectedPodDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName + "-pdb",
			Namespace:       namespace,
			ResourceVersion: "1",
			Labels:          getLabels(),
		},
		TypeMeta: metav1.TypeMeta{Kind: "PodDisruptionBudget", APIVersion: "policy/v1beta1"},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailablePDB,
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
		},
	}

	JustBeforeEach(func() {
		defaultDepWaiter = NewExtAuthServer(c, namespace, image, replicas)
	})

	Describe("#Deploy", func() {
		It("succeeds", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actualDeployment := &appsv1.Deployment{}
			Expect(c.Get(ctx, kutil.Key(expectedDeployment.Namespace, expectedDeployment.Name), actualDeployment)).To(Succeed())
			Expect(actualDeployment).To(DeepEqual(expectedDeployment))

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, kutil.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule).To(DeepEqual(expectedDestinationRule))

			actualService := &corev1.Service{}
			Expect(c.Get(ctx, kutil.Key(expectedService.Namespace, expectedService.Name), actualService)).To(Succeed())
			Expect(actualService).To(DeepEqual(expectedService))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, kutil.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), actualVirtualService)).To(Succeed())
			Expect(actualVirtualService).To(DeepEqual(expectedVirtualService))

			actualVpa := &autoscalingv1beta2.VerticalPodAutoscaler{}
			Expect(c.Get(ctx, kutil.Key(expectedVpa.Namespace, expectedVpa.Name), actualVpa)).To(Succeed())
			Expect(actualVpa).To(DeepEqual(expectedVpa))

			actualPodDisruptionBudget := &policyv1beta1.PodDisruptionBudget{}
			Expect(c.Get(ctx, kutil.Key(expectedPodDisruptionBudget.Namespace, expectedPodDisruptionBudget.Name), actualPodDisruptionBudget)).To(Succeed())
			Expect(actualPodDisruptionBudget).To(DeepEqual(expectedPodDisruptionBudget))

		})

		It("destroy succeeds", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, kutil.Key(expectedDeployment.Namespace, expectedDeployment.Name), &appsv1.Deployment{})).To(Succeed())
			Expect(c.Get(ctx, kutil.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), &istionetworkingv1beta1.DestinationRule{})).To(Succeed())
			Expect(c.Get(ctx, kutil.Key(expectedService.Namespace, expectedService.Name), &corev1.Service{})).To(Succeed())
			Expect(c.Get(ctx, kutil.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), &istionetworkingv1beta1.VirtualService{})).To(Succeed())
			Expect(c.Get(ctx, kutil.Key(expectedVpa.Namespace, expectedVpa.Name), &autoscalingv1beta2.VerticalPodAutoscaler{})).To(Succeed())
			Expect(c.Get(ctx, kutil.Key(expectedPodDisruptionBudget.Namespace, expectedPodDisruptionBudget.Name), &policyv1beta1.PodDisruptionBudget{})).To(Succeed())

			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, kutil.Key(expectedDeployment.Namespace, expectedDeployment.Name), &appsv1.Deployment{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kutil.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kutil.Key(expectedService.Namespace, expectedService.Name), &corev1.Service{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kutil.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kutil.Key(expectedVpa.Namespace, expectedVpa.Name), &autoscalingv1beta2.VerticalPodAutoscaler{})).To(BeNotFoundError())
			Expect(c.Get(ctx, kutil.Key(expectedPodDisruptionBudget.Namespace, expectedPodDisruptionBudget.Name), &policyv1beta1.PodDisruptionBudget{})).To(BeNotFoundError())
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
