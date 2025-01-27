// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authzserver_test

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	vpnauthzserver "github.com/gardener/gardener/pkg/component/networking/vpn/authzserver"
	"github.com/gardener/gardener/pkg/component/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ExtAuthzServer", func() {
	var (
		ctx context.Context
		c   client.Client

		defaultDepWaiter component.DeployWaiter
		namespace        = "shoot--foo--bar"

		kubernetesVersion *semver.Version
		image             = "some-image"
		maxSurge          = intstr.FromInt32(100)
		maxUnavailable    = intstr.FromInt32(0)
		maxUnavailablePDB = intstr.FromInt32(1)
		vpaUpdateMode     = vpaautoscalingv1.UpdateModeAuto

		deploymentName = "reversed-vpn-auth-server"
		serviceName    = "reversed-vpn-auth-server"
		vpaName        = fmt.Sprintf("%s-vpa", "reversed-vpn-auth-server")

		expectedDeployment          *appsv1.Deployment
		expectedDestinationRule     *istionetworkingv1beta1.DestinationRule
		expectedService             *corev1.Service
		expectedVirtualService      *istionetworkingv1beta1.VirtualService
		expectedVpa                 *vpaautoscalingv1.VerticalPodAutoscaler
		expectedPodDisruptionBudget *policyv1.PodDisruptionBudget
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

		kubernetesVersion = semver.MustParse("1.26")

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
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
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
						AutomountServiceAccountToken: ptr.To(false),
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
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
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
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": deploymentName,
				},
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       "grpc-authz",
						Port:       9001,
						TargetPort: intstr.FromInt32(9001),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}

		expectedVirtualService = &istionetworkingv1beta1.VirtualService{
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

		expectedPodDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            deploymentName + "-pdb",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels: map[string]string{
					"app": deploymentName,
				},
			},
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
		defaultDepWaiter = vpnauthzserver.New(c, namespace, image, kubernetesVersion)
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actualDeployment := &appsv1.Deployment{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDeployment.Namespace, Name: expectedDeployment.Name}, actualDeployment)).To(Succeed())
			Expect(actualDeployment).To(DeepEqual(expectedDeployment))

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule).To(BeComparableTo(expectedDestinationRule, test.CmpOptsForDestinationRule()))

			actualService := &corev1.Service{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedService.Namespace, Name: expectedService.Name}, actualService)).To(Succeed())
			Expect(actualService).To(DeepEqual(expectedService))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, actualVirtualService)).To(Succeed())
			Expect(actualVirtualService).To(BeComparableTo(expectedVirtualService, test.CmpOptsForVirtualService()))

			actualVpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVpa.Namespace, Name: expectedVpa.Name}, actualVpa)).To(Succeed())
			Expect(actualVpa).To(DeepEqual(expectedVpa))
		})

		Context("Kubernetes version < 1.26", func() {
			BeforeEach(func() {
				kubernetesVersion = semver.MustParse("1.25")
			})

			It("should successfully deploy all the components", func() {
				actualPodDisruptionBudget := &policyv1.PodDisruptionBudget{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedPodDisruptionBudget.Namespace, Name: expectedPodDisruptionBudget.Name}, actualPodDisruptionBudget)).To(Succeed())
				Expect(actualPodDisruptionBudget).To(DeepEqual(expectedPodDisruptionBudget))
			})
		})

		Context("Kubernetes version >= 1.26", func() {
			It("should successfully deploy all the components", func() {
				unhealthyPodEvictionPolicyAlwatysAllow := policyv1.AlwaysAllow
				expectedPodDisruptionBudget.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwatysAllow

				actualPodDisruptionBudget := &policyv1.PodDisruptionBudget{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedPodDisruptionBudget.Namespace, Name: expectedPodDisruptionBudget.Name}, actualPodDisruptionBudget)).To(Succeed())
				Expect(actualPodDisruptionBudget).To(DeepEqual(expectedPodDisruptionBudget))
			})
		})
	})

	Describe("#Destroy", func() {
		JustBeforeEach(func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDeployment.Namespace, Name: expectedDeployment.Name}, &appsv1.Deployment{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, &istionetworkingv1beta1.DestinationRule{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedService.Namespace, Name: expectedService.Name}, &corev1.Service{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, &istionetworkingv1beta1.VirtualService{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVpa.Namespace, Name: expectedVpa.Name}, &vpaautoscalingv1.VerticalPodAutoscaler{})).To(Succeed())
		})
		AfterEach(func() {
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDeployment.Namespace, Name: expectedDeployment.Name}, &appsv1.Deployment{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedService.Namespace, Name: expectedService.Name}, &corev1.Service{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVpa.Namespace, Name: expectedVpa.Name}, &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
		})

		It("should successfully delete all the components", func() {
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedPodDisruptionBudget.Namespace, Name: expectedPodDisruptionBudget.Name}, &policyv1.PodDisruptionBudget{})).To(Succeed())
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedPodDisruptionBudget.Namespace, Name: expectedPodDisruptionBudget.Name}, &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
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
