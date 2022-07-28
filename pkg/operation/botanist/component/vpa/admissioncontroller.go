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

package vpa

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
)

const (
	admissionController                  = "vpa-admission-controller"
	admissionControllerServiceName       = "vpa-webhook"
	admissionControllerServicePort int32 = 443
	admissionControllerPort              = 10250

	volumeMountPathCertificates = "/etc/tls-certs"
	volumeNameCertificates      = "vpa-tls-certs"
)

// ValuesAdmissionController is a set of configuration values for the vpa-admission-controller.
type ValuesAdmissionController struct {
	// Image is the container image.
	Image string
	// Replicas is the number of pod replicas.
	Replicas int32
}

func (v *vpa) admissionControllerResourceConfigs() component.ResourceConfigs {
	var (
		clusterRole        = v.emptyClusterRole("admission-controller")
		clusterRoleBinding = v.emptyClusterRoleBinding("admission-controller")
		service            = v.emptyService(admissionControllerServiceName)
		deployment         = v.emptyDeployment(admissionController)
		vpa                = v.emptyVerticalPodAutoscaler(admissionController)
	)

	configs := component.ResourceConfigs{
		{Obj: clusterRole, Class: component.Application, MutateFn: func() { v.reconcileAdmissionControllerClusterRole(clusterRole) }},
		{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() {
			v.reconcileAdmissionControllerClusterRoleBinding(clusterRoleBinding, clusterRole, admissionController)
		}},
		{Obj: service, Class: component.Runtime, MutateFn: func() { v.reconcileAdmissionControllerService(service) }},
		{Obj: vpa, Class: component.Runtime, MutateFn: func() { v.reconcileAdmissionControllerVPA(vpa, deployment) }},
	}

	if v.values.ClusterType == component.ClusterTypeSeed {
		serviceAccount := v.emptyServiceAccount(admissionController)
		configs = append(configs,
			component.ResourceConfig{Obj: serviceAccount, Class: component.Application, MutateFn: func() { v.reconcileAdmissionControllerServiceAccount(serviceAccount) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileAdmissionControllerDeployment(deployment, &serviceAccount.Name) }},
		)
	} else {
		networkPolicy := v.emptyNetworkPolicy("allow-kube-apiserver-to-vpa-admission-controller")
		configs = append(configs,
			component.ResourceConfig{Obj: networkPolicy, Class: component.Runtime, MutateFn: func() { v.reconcileAdmissionControllerNetworkPolicy(networkPolicy) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileAdmissionControllerDeployment(deployment, nil) }},
		)
	}

	return configs
}

func (v *vpa) reconcileAdmissionControllerServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = getRoleLabel()
	serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
}

func (v *vpa) reconcileAdmissionControllerClusterRole(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "configmaps", "nodes", "limitranges"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"admissionregistration.k8s.io"},
			Resources: []string{"mutatingwebhookconfigurations"},
			Verbs:     []string{"create", "delete", "get", "list"},
		},
		{
			APIGroups: []string{"poc.autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"create", "update", "get", "list", "watch"},
		},
	}
}

func (v *vpa) reconcileAdmissionControllerClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole, serviceAccountName string) {
	clusterRoleBinding.Labels = getRoleLabel()
	clusterRoleBinding.Annotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      serviceAccountName,
		Namespace: v.serviceAccountNamespace(),
	}}
}

func (v *vpa) reconcileAdmissionControllerService(service *corev1.Service) {
	service.Spec.Selector = getAppLabel(admissionController)
	desiredPorts := []corev1.ServicePort{
		{
			Port:       admissionControllerServicePort,
			TargetPort: intstr.FromInt(admissionControllerPort),
		},
	}
	service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, "")
}

func (v *vpa) reconcileAdmissionControllerNetworkPolicy(networkPolicy *networkingv1.NetworkPolicy) {
	protocol := corev1.ProtocolTCP
	port := intstr.FromInt(admissionControllerPort)

	networkPolicy.Annotations = map[string]string{v1beta1constants.GardenerDescription: "Allows Egress from shoot's kube-apiserver pods to the VPA admission controller."}
	networkPolicy.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: kubeapiserver.GetLabels(),
		},
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: getAppLabel(admissionController),
				},
			}},
			Ports: []networkingv1.NetworkPolicyPort{{
				Protocol: &protocol,
				Port:     &port,
			}},
		}},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
	}
}

func (v *vpa) reconcileAdmissionControllerDeployment(deployment *appsv1.Deployment, serviceAccountName *string) {
	priorityClassName := v1beta1constants.PriorityClassNameSeedSystem800
	if v.values.ClusterType == component.ClusterTypeShoot {
		priorityClassName = v1beta1constants.PriorityClassNameShootControlPlane200
	}

	deployment.Labels = v.getDeploymentLabels(admissionController)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             &v.values.AdmissionController.Replicas,
		RevisionHistoryLimit: pointer.Int32(2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(admissionController)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: getAllLabels(admissionController),
			},
			Spec: corev1.PodSpec{
				PriorityClassName: priorityClassName,
				Containers: []corev1.Container{{
					Name:            "admission-controller",
					Image:           v.values.AdmissionController.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         v.computeAdmissionControllerCommands(),
					Args: []string{
						"--v=2",
						"--stderrthreshold=info",
						fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathCertificates, secretutils.DataKeyCertificateBundle),
						fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathCertificates, secretutils.DataKeyCertificate),
						fmt.Sprintf("--tls-private-key=%s/%s", volumeMountPathCertificates, secretutils.DataKeyPrivateKey),
						"--address=:8944",
						fmt.Sprintf("--port=%d", admissionControllerPort),
						"--register-webhook=false",
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("30m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
					},
					Ports: []corev1.ContainerPort{{
						ContainerPort: admissionControllerPort,
					}},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      volumeNameCertificates,
						MountPath: volumeMountPathCertificates,
						ReadOnly:  true,
					}},
				}},
				Volumes: []corev1.Volume{{
					Name: volumeNameCertificates,
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: pointer.Int32(420),
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: v.caSecretName,
										},
										Items: []corev1.KeyToPath{{
											Key:  secretutils.DataKeyCertificateBundle,
											Path: secretutils.DataKeyCertificateBundle,
										}},
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: v.serverSecretName,
										},
										Items: []corev1.KeyToPath{
											{
												Key:  secretutils.DataKeyCertificate,
												Path: secretutils.DataKeyCertificate,
											},
											{
												Key:  secretutils.DataKeyPrivateKey,
												Path: secretutils.DataKeyPrivateKey,
											},
										},
									},
								},
							},
						},
					},
				}},
			},
		},
	}

	if v.values.ClusterType == component.ClusterTypeShoot {
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			v1beta1constants.LabelNetworkPolicyFromShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToShootAPIServer:   v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	v.injectAPIServerConnectionSpec(deployment, admissionController, serviceAccountName)
}

func (v *vpa) reconcileAdmissionControllerVPA(vpa *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) {
	updateMode := vpaautoscalingv1.UpdateModeAuto
	controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly

	vpa.Spec = vpaautoscalingv1.VerticalPodAutoscalerSpec{
		TargetRef: &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       deployment.Name,
		},
		UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &updateMode},
		ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName:    "*",
					ControlledValues: &controlledValues,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		},
	}
}

func (v *vpa) computeAdmissionControllerCommands() []string {
	// TODO(shafeeqes): add --kubeconfig flag also, after https://github.com/kubernetes/autoscaler/issues/4844 is fixed.
	out := []string{"./admission-controller"}

	return out
}
