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

package kubestatemetrics

import (
	"fmt"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheus/cache"
	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (k *kubeStateMetrics) getResourceConfigs(genericTokenKubeconfigSecretName string, shootAccessSecret *gardenerutils.AccessSecret) component.ResourceConfigs {
	var (
		clusterRole        = k.emptyClusterRole()
		clusterRoleBinding = k.emptyClusterRoleBinding()
		service            = k.emptyService()
		deployment         = k.emptyDeployment()
		vpa                = k.emptyVerticalPodAutoscaler()
		pdb                = k.emptyPodDisruptionBudget()
		serviceMonitor     = k.emptyServiceMonitor()

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
			component.ResourceConfig{Obj: pdb, Class: component.Runtime, MutateFn: func() { k.reconcilePodDisruptionBudget(pdb, deployment) }},
			component.ResourceConfig{Obj: serviceMonitor, Class: component.Runtime, MutateFn: func() { k.reconcileServiceMonitor(serviceMonitor) }},
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
	serviceAccount.AutomountServiceAccountToken = ptr.To(false)
}

func (k *kubeStateMetrics) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeStateMetrics, k.namespace)
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
		{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	if k.values.ClusterType == component.ClusterTypeSeed {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"autoscaling"},
			Resources: []string{"horizontalpodautoscalers"},
			Verbs:     []string{"list", "watch"},
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

	metricsPort := networkingv1.NetworkPolicyPort{
		Port:     utils.IntStrPtrFromInt32(port),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}

	switch k.values.ClusterType {
	case component.ClusterTypeSeed:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, metricsPort))
	case component.ClusterTypeShoot:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, metricsPort))
	}

	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.Selector = k.getLabels()
	service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, []corev1.ServicePort{
		{
			Name:       portNameMetrics,
			Port:       80,
			TargetPort: intstr.FromInt32(port),
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
	shootAccessSecret *gardenerutils.AccessSecret,
) {
	var (
		maxUnavailable = intstr.FromInt32(1)

		deploymentLabels = k.getLabels()
		podLabels        = map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
		}
		args = []string{
			fmt.Sprintf("--port=%d", port),
			"--telemetry-port=8081",
		}
	)

	if k.values.ClusterType == component.ClusterTypeSeed {
		deploymentLabels[v1beta1constants.LabelRole] = v1beta1constants.LabelMonitoring
		podLabels = utils.MergeStringMaps(podLabels, deploymentLabels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})
		args = append(args,
			"--resources=deployments,pods,statefulsets,nodes,verticalpodautoscalers,horizontalpodautoscalers,persistentvolumeclaims,replicasets,namespaces",
			"--metric-labels-allowlist=nodes=[*]",
			"--metric-annotations-allowlist=namespaces=[shoot.gardener.cloud/uid]",
			fmt.Sprintf("--metric-allowlist=%s", strings.Join(cachePrometheusAllowedMetrics, ",")),
		)
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		deploymentLabels[v1beta1constants.GardenRole] = v1beta1constants.LabelMonitoring
		podLabels = utils.MergeStringMaps(podLabels, deploymentLabels, map[string]string{
			gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
		})
		args = append(args,
			"--resources=daemonsets,deployments,nodes,pods,statefulsets,verticalpodautoscalers,replicasets",
			"--namespaces="+metav1.NamespaceSystem,
			"--kubeconfig="+gardenerutils.PathGenericKubeconfig,
			"--metric-labels-allowlist=nodes=[*]",
		)
	}

	deployment.Labels = deploymentLabels
	deployment.Spec.Replicas = &k.values.Replicas
	deployment.Spec.RevisionHistoryLimit = ptr.To(int32(2))
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
							Port: intstr.FromInt32(port),
						},
					},
					InitialDelaySeconds: 5,
					TimeoutSeconds:      5,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt32(port),
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
			PriorityClassName: k.values.PriorityClassName,
		},
	}

	if k.values.ClusterType == component.ClusterTypeSeed {
		deployment.Spec.Template.Spec.ServiceAccountName = serviceAccount.Name
	}
	if k.values.ClusterType == component.ClusterTypeShoot {
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = ptr.To(false)
		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecretName, shootAccessSecret.Secret.Name))
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
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
		},
	}
}

func (k *kubeStateMetrics) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "kube-state-metrics-pdb", Namespace: k.namespace}}
}

func (k *kubeStateMetrics) reconcilePodDisruptionBudget(podDisruptionBudget *policyv1.PodDisruptionBudget, deployment *appsv1.Deployment) {
	podDisruptionBudget.Labels = k.getLabels()
	podDisruptionBudget.Spec = policyv1.PodDisruptionBudgetSpec{
		MaxUnavailable: utils.IntStrPtrFromInt32(1),
		Selector:       deployment.Spec.Selector,
	}

	kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, k.values.KubernetesVersion)
}

func (k *kubeStateMetrics) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta("kube-state-metrics", k.namespace, cache.Label)}
}

func (k *kubeStateMetrics) reconcileServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor) {
	serviceMonitor.Labels = monitoringutils.Labels(cache.Label)
	serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
		Selector: metav1.LabelSelector{MatchLabels: k.getLabels()},
		Endpoints: []monitoringv1.Endpoint{{
			Port: portNameMetrics,
			RelabelConfigs: []*monitoringv1.RelabelConfig{
				{
					TargetLabel: "instance",
					Replacement: "kube-state-metrics",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_label_type"},
					Regex:        `(.+)`,
					TargetLabel:  "type",
					Replacement:  `${1}`,
				},
			},
			MetricRelabelConfigs: append([]*monitoringv1.RelabelConfig{{
				SourceLabels: []monitoringv1.LabelName{"pod"},
				Regex:        `^.+\.tf-pod.+$`,
				Action:       "drop",
			}}, monitoringutils.StandardMetricRelabelConfig(cachePrometheusAllowedMetrics...)...),
		}},
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
