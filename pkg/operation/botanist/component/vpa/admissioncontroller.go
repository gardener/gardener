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
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	admissionController                  = "vpa-admission-controller"
	admissionControllerServicePort int32 = 443
	admissionControllerPort        int32 = 10250
)

// ValuesAdmissionController is a set of configuration values for the vpa-admission-controller.
type ValuesAdmissionController struct {
	// Image is the container image.
	Image string
}

func (v *vpa) admissionControllerResourceConfigs() resourceConfigs {
	var (
		clusterRole        = v.emptyClusterRole("admission-controller")
		clusterRoleBinding = v.emptyClusterRoleBinding("admission-controller")
		service            = v.emptyService("vpa-webhook")
	)

	configs := resourceConfigs{
		{obj: clusterRole, class: application, mutateFn: func() { v.reconcileAdmissionControllerClusterRole(clusterRole) }},
		{obj: clusterRoleBinding, class: application, mutateFn: func() {
			v.reconcileAdmissionControllerClusterRoleBinding(clusterRoleBinding, clusterRole, admissionController)
		}},
		{obj: service, class: runtime, mutateFn: func() { v.reconcileAdmissionControllerService(service) }},
	}

	if v.values.ClusterType == ClusterTypeSeed {
		serviceAccount := v.emptyServiceAccount(admissionController)
		configs = append(configs,
			resourceConfig{obj: serviceAccount, class: application, mutateFn: func() { v.reconcileAdmissionControllerServiceAccount(serviceAccount) }},
		)
	} else {
		configs = append(configs)
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
			TargetPort: intstr.FromInt(int(admissionControllerPort)),
		},
	}
	service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, "")
}
