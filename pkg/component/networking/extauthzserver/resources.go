// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extauthzserver

import (
	"fmt"

	istioapinetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/istio"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

func (e *extAuthzServer) getService(isShootNamespace bool) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.getPrefix() + svcName,
			Namespace: e.namespace,
			Annotations: map[string]string{
				"networking.istio.io/exportTo": "*",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: e.getLabels(),
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       Port,
					TargetPort: intstr.FromInt32(Port),
				},
			},
		},
	}

	namespaceSelectors := []metav1.LabelSelector{{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}}}

	if isShootNamespace {
		metav1.SetMetaDataAnnotation(&svc.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)

		namespaceSelectors = append(namespaceSelectors,
			metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: v1beta1constants.LabelExposureClassHandlerName, Operator: metav1.LabelSelectorOpExists}}},
		)
	}

	utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(svc, namespaceSelectors...))

	return svc
}

func (e *extAuthzServer) getDeployment(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.getPrefix() + v1beta1constants.DeploymentNameExtAuthzServer,
			Namespace: e.namespace,
			Labels:    e.getLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: e.getLabels(),
			},
			Replicas: &e.values.Replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: e.getLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           e.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--grpc-reflection",
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(Port),
									},
								},
								SuccessThreshold: 1,
								FailureThreshold: 2,
								PeriodSeconds:    10,
								TimeoutSeconds:   5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(Port),
									},
								},
								SuccessThreshold: 1,
								FailureThreshold: 2,
								PeriodSeconds:    10,
								TimeoutSeconds:   5,
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: Port,
									Name:          "grpc",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					PriorityClassName: e.values.PriorityClassName,
					Volumes:           volumes,
				},
			},
		},
	}
}

func (e *extAuthzServer) getEnvoyFilter(
	configPatches []*istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch,
	ownerReference *metav1.OwnerReference,
) *istionetworkingv1alpha3.EnvoyFilter {
	// Currently, all observability components are exposed via the same istio ingress gateway.
	// When zonal gateways or exposure classes should be considered, the namespace needs to be dynamic.
	ingressNamespace := e.getPrefix() + v1beta1constants.DefaultSNIIngressNamespace

	return &istionetworkingv1alpha3.EnvoyFilter{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s%s", e.namespace, e.getPrefix(), name),
			Namespace:       ingressNamespace,
			OwnerReferences: []metav1.OwnerReference{*ownerReference},
		},
		Spec: istioapinetworkingv1alpha3.EnvoyFilter{
			ConfigPatches: configPatches,
		},
	}
}

func (e *extAuthzServer) getDestinationRule(destinationHost string, secretName string) (*istionetworkingv1beta1.DestinationRule, error) {
	destinationRule := &istionetworkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: e.getPrefix() + v1beta1constants.DeploymentNameExtAuthzServer, Namespace: e.namespace}}
	//	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, e.getLabels(), destinationHost)(); err != nil {
	if err := istio.DestinationRuleWithTLSTermination(
		destinationRule,
		e.getLabels(),
		destinationHost,
		destinationHost,
		secretName,
		istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE,
	)(); err != nil {
		return nil, fmt.Errorf("failed to create destination rule: %w", err)
	}
	destinationRule.Spec.TrafficPolicy.ConnectionPool.Http = nil

	return destinationRule, nil
}

func (e *extAuthzServer) getTLSSecret(caSecret *corev1.Secret, secretName string, ownerReference *metav1.OwnerReference) *corev1.Secret {
	// Currently, all observability components are exposed via the same istio ingress gateway.
	// When zonal gateways or exposure classes should be considered, the namespace needs to be dynamic.
	ingressNamespace := e.getPrefix() + v1beta1constants.DefaultSNIIngressNamespace

	// Istio expects the secret in the istio ingress gateway namespace => copy certificate to istio namespace
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       ingressNamespace,
			Labels:          e.getLabels(),
			OwnerReferences: []metav1.OwnerReference{*ownerReference},
		},
		Data: map[string][]byte{
			"cacert": caSecret.Data[secretsutils.DataKeyCertificateCA],
		},
	}
}

func (e *extAuthzServer) getVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.getPrefix() + name,
			Namespace: e.namespace,
			Labels:    e.getLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       e.getPrefix() + v1beta1constants.DeploymentNameExtAuthzServer,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeRecreate),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
					ContainerName:    name,
					ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
				}},
			},
		},
	}
}
