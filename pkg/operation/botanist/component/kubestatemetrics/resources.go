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

package kubestatemetrics

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
)

func (k *kubeStateMetrics) getResourceConfigs(genericTokenKubeconfigSecretName string, shootAccessSecret *gutil.ShootAccessSecret) component.ResourceConfigs {
	var (
		clusterRole        = k.emptyClusterRole()
		clusterRoleBinding = k.emptyClusterRoleBinding()
		service            = k.emptyService()
		deployment         = k.emptyDeployment()
		vpa                = k.emptyVerticalPodAutoscaler()

		configs = component.ResourceConfigs{
			{Obj: clusterRole, Class: component.Application, MutateFn: func() { k.reconcileClusterRole(clusterRole) }},
			{Obj: service, Class: component.Runtime, MutateFn: func() { k.reconcileService(service) }},
			{Obj: vpa, Class: component.Runtime, MutateFn: func() { k.reconcileVerticalPodAutoscaler(vpa, deployment) }},
		}
	)

	if k.values.ClusterType == component.ClusterTypeSeed {
		serviceAccount := k.emptyServiceAccount()

		configs = append(configs,
			component.ResourceConfig{Obj: serviceAccount, Class: component.Runtime, MutateFn: func() { k.reconcileServiceAccount(serviceAccount) }},
			component.ResourceConfig{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() { k.reconcileClusterRoleBinding(clusterRoleBinding, clusterRole, serviceAccount) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { k.reconcileDeployment(deployment, serviceAccount, "", nil) }},
		)
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		configs = append(configs,
			component.ResourceConfig{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() {
				k.reconcileClusterRoleBinding(clusterRoleBinding, clusterRole, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecret.ServiceAccountName, Namespace: metav1.NamespaceSystem}})
			}},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { k.reconcileDeployment(deployment, nil, genericTokenKubeconfigSecretName, shootAccessSecret) }},
		)
	}

	return configs
}

func (k *kubeStateMetrics) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "kube-state-metrics", Namespace: k.namespace}}
}

func (k *kubeStateMetrics) reconcileServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = k.getLabels()
	serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
}

func (k *kubeStateMetrics) newShootAccessSecret() *gutil.ShootAccessSecret {
	return gutil.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeStateMetrics, k.namespace)
}

func (k *kubeStateMetrics) emptyClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:monitoring:" + k.nameSuffix()}}
}

func (k *kubeStateMetrics) reconcileClusterRole(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = k.getLabels()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{
				"nodes",
				"pods",
				"services",
				"resourcequotas",
				"replicationcontrollers",
				"limitranges",
				"persistentvolumeclaims",
				"namespaces",
			},
			Verbs: []string{"list", "watch"},
		},
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
			Verbs:     []string{"list", "watch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"cronjobs", "jobs"},
			Verbs:     []string{"list", "watch"},
		},
	}

	if k.values.ClusterType == component.ClusterTypeSeed {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"autoscaling"},
			Resources: []string{"horizontalpodautoscalers"},
			Verbs:     []string{"list", "watch"},
		})
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch"},
		})
	}
}

func (k *kubeStateMetrics) emptyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:monitoring:" + k.nameSuffix()}}
}

func (k *kubeStateMetrics) reconcileClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole, serviceAccount *corev1.ServiceAccount) {
	clusterRoleBinding.Labels = k.getLabels()
	clusterRoleBinding.Annotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      serviceAccount.Name,
		Namespace: serviceAccount.Namespace,
	}}
}

func (k *kubeStateMetrics) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "kube-state-metrics", Namespace: k.namespace}}
}

func (k *kubeStateMetrics) reconcileService(service *corev1.Service) {
	service.Labels = k.getLabels()
	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.Selector = k.getLabels()
	service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       80,
			TargetPort: intstr.FromInt(port),
			Protocol:   corev1.ProtocolTCP,
		},
	}, corev1.ServiceTypeClusterIP)
}

func (k *kubeStateMetrics) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-state-metrics", Namespace: k.namespace}}
}

func (k *kubeStateMetrics) reconcileDeployment(
	deployment *appsv1.Deployment,
	serviceAccount *corev1.ServiceAccount,
	genericTokenKubeconfigSecretName string,
	shootAccessSecret *gutil.ShootAccessSecret,
) {
	var (
		maxUnavailable = intstr.FromInt(1)

		deploymentLabels = k.getLabels()
		podLabels        = map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:          v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyFromPrometheus: v1beta1constants.LabelNetworkPolicyAllowed,
		}
		args = []string{
			fmt.Sprintf("--port=%d", port),
			"--telemetry-port=8081",
		}
	)

	if k.values.ClusterType == component.ClusterTypeSeed {
		deploymentLabels[v1beta1constants.LabelRole] = v1beta1constants.LabelMonitoring
		podLabels = utils.MergeStringMaps(podLabels, deploymentLabels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToSeedAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})
		args = append(args,
			"--resources=deployments,pods,statefulsets,nodes,horizontalpodautoscalers,persistentvolumeclaims,replicasets",
		)
	}

	priorityClassName := v1beta1constants.PriorityClassNameSeedSystem600
	if k.values.ClusterType == component.ClusterTypeShoot {
		priorityClassName = v1beta1constants.PriorityClassNameShootControlPlane100
		deploymentLabels[v1beta1constants.GardenRole] = v1beta1constants.LabelMonitoring
		podLabels = utils.MergeStringMaps(podLabels, deploymentLabels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})
		args = append(args,
			"--resources=daemonsets,deployments,nodes,pods,statefulsets,verticalpodautoscalers,replicasets",
			"--namespaces="+metav1.NamespaceSystem,
			"--kubeconfig="+gutil.PathGenericKubeconfig,
		)
	}

	deployment.Labels = deploymentLabels
	deployment.Spec.Replicas = &k.values.Replicas
	deployment.Spec.RevisionHistoryLimit = pointer.Int32(2)
	deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: k.getLabels()}
	deployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &maxUnavailable,
		},
	}
	deployment.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: podLabels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            containerName,
				Image:           k.values.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            args,
				Ports: []corev1.ContainerPort{{
					Name:          "metrics",
					ContainerPort: port,
					Protocol:      corev1.ProtocolTCP,
				}},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(port),
						},
					},
					InitialDelaySeconds: 5,
					TimeoutSeconds:      5,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(port),
						},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       30,
					SuccessThreshold:    1,
					FailureThreshold:    3,
					TimeoutSeconds:      5,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			}},
			PriorityClassName: priorityClassName,
		},
	}

	if k.values.ClusterType == component.ClusterTypeSeed {
		deployment.Spec.Template.Spec.ServiceAccountName = serviceAccount.Name
	}
	if k.values.ClusterType == component.ClusterTypeShoot {
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = pointer.Bool(false)
		utilruntime.Must(gutil.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecretName, shootAccessSecret.Secret.Name))
	}
}

func (k *kubeStateMetrics) emptyVerticalPodAutoscaler() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-state-metrics-vpa", Namespace: k.namespace}}
}

func (k *kubeStateMetrics) reconcileVerticalPodAutoscaler(vpa *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) {
	var (
		updateMode       = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	)

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
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
		},
	}
}

func (k *kubeStateMetrics) getLabels() map[string]string {
	t := "seed"
	if k.values.ClusterType == component.ClusterTypeShoot {
		t = "shoot"
	}

	return map[string]string{
		labelKeyComponent: labelValueComponent,
		labelKeyType:      t,
	}
}

func (k *kubeStateMetrics) nameSuffix() string {
	suffix := "kube-state-metrics"
	if k.values.ClusterType == component.ClusterTypeShoot {
		return suffix
	}
	return suffix + "-seed"
}
