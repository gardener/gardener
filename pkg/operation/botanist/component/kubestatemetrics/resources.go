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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func (k *kubeStateMetrics) getResourceConfigs(shootAccessSecret *gutil.ShootAccessSecret) component.ResourceConfigs {
	var (
		clusterRole        = k.emptyClusterRole()
		clusterRoleBinding = k.emptyClusterRoleBinding()
		service            = k.emptyService()

		configs = component.ResourceConfigs{
			{Obj: clusterRole, Class: component.Application, MutateFn: func() { k.reconcileClusterRole(clusterRole) }},
			{Obj: service, Class: component.Runtime, MutateFn: func() { k.reconcileService(service) }},
		}
	)

	if k.values.ClusterType == component.ClusterTypeSeed {
		serviceAccount := k.emptyServiceAccount()

		configs = append(configs,
			component.ResourceConfig{Obj: serviceAccount, Class: component.Runtime, MutateFn: func() { k.reconcileServiceAccount(serviceAccount) }},
			component.ResourceConfig{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() { k.reconcileClusterRoleBinding(clusterRoleBinding, clusterRole, serviceAccount) }},
		)
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		configs = append(configs,
			component.ResourceConfig{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() {
				k.reconcileClusterRoleBinding(clusterRoleBinding, clusterRole, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecret.ServiceAccountName, Namespace: metav1.NamespaceSystem}})
			}},
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
