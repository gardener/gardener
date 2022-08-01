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
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
)

const (
	exporter                      = "vpa-exporter"
	exporterPortMetrics     int32 = 9570
	exporterPortNameMetrics       = "metrics"
)

// ValuesExporter is a set of configuration values for the vpa-exporter.
type ValuesExporter struct {
	// Image is the container image.
	Image string
}

func (v *vpa) exporterResourceConfigs() component.ResourceConfigs {
	var (
		service            = v.emptyService(exporter)
		serviceAccount     = v.emptyServiceAccount(exporter)
		clusterRole        = v.emptyClusterRole("exporter")
		clusterRoleBinding = v.emptyClusterRoleBinding("exporter")
		deployment         = v.emptyDeployment(exporter)
		vpa                = v.emptyVerticalPodAutoscaler(exporter + "-vpa")
	)

	return component.ResourceConfigs{
		{Obj: service, Class: component.Runtime, MutateFn: func() { v.reconcileExporterService(service) }},
		{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileExporterDeployment(deployment, serviceAccount) }},
		{Obj: vpa, Class: component.Runtime, MutateFn: func() { v.reconcileExporterVPA(vpa, deployment) }},
		{Obj: serviceAccount, Class: component.Application, MutateFn: func() { v.reconcileExporterServiceAccount(serviceAccount) }},
		{Obj: clusterRole, Class: component.Application, MutateFn: func() { v.reconcileExporterClusterRole(clusterRole) }},
		{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() { v.reconcileExporterClusterRoleBinding(clusterRoleBinding, clusterRole, serviceAccount) }},
	}
}

func (v *vpa) reconcileExporterService(service *corev1.Service) {
	service.Labels = getAllLabels(exporter)
	service.Spec = corev1.ServiceSpec{
		Type:            corev1.ServiceTypeClusterIP,
		SessionAffinity: corev1.ServiceAffinityNone,
		Selector:        getAppLabel(exporter),
	}

	desiredPorts := []corev1.ServicePort{
		{
			Name:       exporterPortNameMetrics,
			Protocol:   corev1.ProtocolTCP,
			Port:       exporterPortMetrics,
			TargetPort: intstr.FromInt(int(exporterPortMetrics)),
		},
	}
	service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
}

func (v *vpa) reconcileExporterServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
}

func (v *vpa) reconcileExporterClusterRole(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{{
		APIGroups: []string{"autoscaling.k8s.io"},
		Resources: []string{"verticalpodautoscalers"},
		Verbs:     []string{"get", "watch", "list"},
	}}
}

func (v *vpa) reconcileExporterClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole, serviceAccount *corev1.ServiceAccount) {
	clusterRoleBinding.Labels = getRoleLabel()
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

func (v *vpa) reconcileExporterDeployment(deployment *appsv1.Deployment, serviceAccount *corev1.ServiceAccount) {
	deployment.Labels = getAllLabels(exporter)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             pointer.Int32(1),
		RevisionHistoryLimit: pointer.Int32(2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(exporter)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: getAllLabels(exporter),
			},
			Spec: corev1.PodSpec{
				PriorityClassName:  v1beta1constants.PriorityClassNameSeedSystem600,
				ServiceAccountName: serviceAccount.Name,
				Containers: []corev1.Container{{
					Name:            "exporter",
					Image:           v.values.Exporter.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"/usr/local/bin/vpa-exporter",
						fmt.Sprintf("--port=%d", exporterPortMetrics),
					},
					Ports: []corev1.ContainerPort{{
						Name:          exporterPortNameMetrics,
						ContainerPort: exporterPortMetrics,
						Protocol:      corev1.ProtocolTCP,
					}},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("30m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},
				}},
			},
		},
	}
}

func (v *vpa) reconcileExporterVPA(vpa *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) {
	updateMode := vpaautoscalingv1.UpdateModeAuto

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
					ContainerName: "*",
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("30m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
			},
		},
	}
}
