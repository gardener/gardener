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
	"context"

	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	exporter                  = "vpa-exporter"
	exporterPortMetrics int32 = 9570
)

// ValuesExporter is a set of configuration values for the vpa-exporter.
type ValuesExporter struct {
	// Image is the container image.
	Image string
}

func (v *vpa) deployExporterResources(ctx context.Context) error {
	var (
		service            = v.emptyService(exporter)
		serviceAccount     = v.emptyServiceAccount(exporter)
		clusterRole        = v.emptyClusterRole("exporter")
		clusterRoleBinding = v.emptyClusterRoleBinding("exporter")

		objToMutateFn = map[client.Object]func(){
			service:            func() { v.reconcileExporterService(service) },
			serviceAccount:     func() { v.reconcileExporterServiceAccount(serviceAccount) },
			clusterRole:        func() { v.reconcileExporterClusterRole(clusterRole) },
			clusterRoleBinding: func() { v.reconcileExporterClusterRoleBinding(clusterRoleBinding, clusterRole, serviceAccount) },
		}
	)

	if v.values.ClusterType == ClusterTypeSeed {
		for obj, mutateFn := range objToMutateFn {
			mutateFn()

			if err := v.registry.Add(obj); err != nil {
				return err
			}
		}

		return nil
	}

	for obj, mutateFn := range objToMutateFn {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, obj, func() error {
			mutateFn()
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (v *vpa) destroyExporterResources(ctx context.Context) error {
	if v.values.ClusterType == ClusterTypeSeed {
		return nil
	}

	return kutil.DeleteObjects(ctx, v.client,
		v.emptyService(exporter),
		v.emptyServiceAccount(exporter),
		v.emptyClusterRole("exporter"),
		v.emptyClusterRoleBinding("exporter"),
	)
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
			Name:       "metrics",
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
