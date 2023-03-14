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

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/constants"
	vpaconstants "github.com/gardener/gardener/pkg/operation/botanist/component/vpa/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	admissionController                  = "vpa-admission-controller"
	admissionControllerServicePort int32 = 443

	volumeMountPathCertificates = "/etc/tls-certs"
	volumeNameCertificates      = "vpa-tls-certs"
)

// ValuesAdmissionController is a set of configuration values for the vpa-admission-controller.
type ValuesAdmissionController struct {
	// Image is the container image.
	Image string
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas is the number of pod replicas.
	Replicas *int32
	// TopologyAwareRoutingEnabled indicates whether topology-aware routing is enabled for the vpa-webhoook service.
	// This value is only applicable for the vpa-admission-controller that is deployed in the Shoot control plane (when ClusterType=shoot).
	TopologyAwareRoutingEnabled bool
}

func (v *vpa) admissionControllerResourceConfigs() component.ResourceConfigs {
	var (
		clusterRole         = v.emptyClusterRole("admission-controller")
		clusterRoleBinding  = v.emptyClusterRoleBinding("admission-controller")
		service             = v.emptyService(vpaconstants.AdmissionControllerServiceName)
		deployment          = v.emptyDeployment(admissionController)
		podDisruptionBudget = v.emptyPodDisruptionBudget(admissionController, version.ConstraintK8sGreaterEqual121.Check(v.values.RuntimeKubernetesVersion))
		vpa                 = v.emptyVerticalPodAutoscaler(admissionController)
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
			component.ResourceConfig{Obj: podDisruptionBudget, Class: component.Runtime, MutateFn: func() { v.reconcilePodDisruptionBudget(podDisruptionBudget, deployment) }},
		)
	} else {
		configs = append(configs,
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileAdmissionControllerDeployment(deployment, nil) }},
			component.ResourceConfig{Obj: podDisruptionBudget, Class: component.Runtime, MutateFn: func() { v.reconcilePodDisruptionBudget(podDisruptionBudget, deployment) }},
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
	topologAwareRoutingEnabled := v.values.AdmissionController.TopologyAwareRoutingEnabled && v.values.ClusterType == component.ClusterTypeShoot
	gardenerutils.ReconcileTopologyAwareRoutingMetadata(service, topologAwareRoutingEnabled)

	if v.values.ClusterType == component.ClusterTypeSeed {
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingFromWorldToPorts, fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, vpaconstants.AdmissionControllerPort))
	}

	service.Spec.Selector = getAppLabel(admissionController)
	desiredPorts := []corev1.ServicePort{
		{
			Port:       admissionControllerServicePort,
			TargetPort: intstr.FromInt(vpaconstants.AdmissionControllerPort),
		},
	}
	service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, "")
}

func (v *vpa) reconcileAdmissionControllerDeployment(deployment *appsv1.Deployment, serviceAccountName *string) {
	deployment.Labels = utils.MergeStringMaps(v.getDeploymentLabels(admissionController), map[string]string{
		resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
	})
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             pointer.Int32(pointer.Int32Deref(v.values.AdmissionController.Replicas, 1)),
		RevisionHistoryLimit: pointer.Int32(2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(admissionController)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getAllLabels(admissionController), map[string]string{
					v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				PriorityClassName: v.values.AdmissionController.PriorityClassName,
				Containers: []corev1.Container{{
					Name:            "admission-controller",
					Image:           v.values.AdmissionController.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         v.computeAdmissionControllerCommands(),
					Args: []string{
						"--v=2",
						"--stderrthreshold=info",
						fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathCertificates, secretsutils.DataKeyCertificateBundle),
						fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathCertificates, secretsutils.DataKeyCertificate),
						fmt.Sprintf("--tls-private-key=%s/%s", volumeMountPathCertificates, secretsutils.DataKeyPrivateKey),
						"--address=:8944",
						fmt.Sprintf("--port=%d", vpaconstants.AdmissionControllerPort),
						"--register-webhook=false",
					},
					LivenessProbe: newDefaultLivenessProbe(),
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("30m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          metricsPortName,
							ContainerPort: 8944,
						},
						{
							ContainerPort: vpaconstants.AdmissionControllerPort,
						},
					},
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
											Key:  secretsutils.DataKeyCertificateBundle,
											Path: secretsutils.DataKeyCertificateBundle,
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
												Key:  secretsutils.DataKeyCertificate,
												Path: secretsutils.DataKeyCertificate,
											},
											{
												Key:  secretsutils.DataKeyPrivateKey,
												Path: secretsutils.DataKeyPrivateKey,
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

	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})

	case component.ClusterTypeShoot:
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
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
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		},
	}
}

func (v *vpa) computeAdmissionControllerCommands() []string {
	out := []string{"./admission-controller"}

	// TODO: add --kubeconfig here (similar to updater, recommender) as soon as support for seeds and shoots < 1.21 is
	//  dropped.

	return out
}
