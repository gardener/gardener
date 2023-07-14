// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/charts"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// DeployManagedResourceForAddons deploys all the ManagedResource CRDs for the gardener-resource-manager.
func (b *Botanist) DeployManagedResourceForAddons(ctx context.Context) error {
	// TODO(shafeeqes): Drop this code in gardener v1.85.
	if err := b.cleanupAddons(ctx); err != nil {
		return err
	}

	renderedChart, err := b.generateCoreAddonsChart()
	if err != nil {
		return fmt.Errorf("error rendering shoot-core chart: %w", err)
	}

	return managedresources.CreateForShoot(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, "shoot-core", managedresources.LabelValueGardener, false, renderedChart.AsSecretData())
}

// generateCoreAddonsChart renders the gardener-resource-manager configuration for the core addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *Botanist) generateCoreAddonsChart() (*chartrenderer.RenderedChart, error) {
	podSecurityPolicies := map[string]interface{}{
		"allowPrivilegedContainers": pointer.BoolDeref(b.Shoot.GetInfo().Spec.Kubernetes.AllowPrivilegedContainers, false),
	}

	values := map[string]interface{}{
		"monitoring":          common.GenerateAddonConfig(map[string]interface{}{}, b.Operation.IsShootMonitoringEnabled()),
		"podsecuritypolicies": common.GenerateAddonConfig(podSecurityPolicies, !b.Shoot.PSPDisabled && !b.Shoot.IsWorkerless),
	}

	return b.ShootClientSet.ChartRenderer().Render(filepath.Join(charts.Path, "shoot-core", "components"), "shoot-core", metav1.NamespaceSystem, values)
}

func (b *Botanist) cleanupAddons(ctx context.Context) error {
	addonsMR := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "addons",
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(addonsMR), addonsMR); err != nil {
		// If the MR is already gone, then nothing to do here
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	data, err := computeResourcesData()
	if err != nil {
		return err
	}

	if err := managedresources.CreateForShoot(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, "addons", managedresources.LabelValueGardener, false, data); err != nil {
		return err
	}

	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(addonsMR), addonsMR); err != nil {
		return err
	}

	if len(addonsMR.Status.Resources) == 0 {
		if err := managedresources.DeleteForShoot(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, "addons"); err != nil {
			return err
		}

		if err := managedresources.WaitUntilDeleted(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, "addons"); err != nil {
			return err
		}
	}

	return nil
}

func computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		kubernetesDashboardNamespace = "kubernetes-dashboard"
		kubernetesDashboardName      = "kubernetes-dashboard"

		dashboardNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardRole = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardName,
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardRoleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardName,
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardClusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardName,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardName,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardServiceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardName,
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardSecretCerts = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "kubernetes-dashboard-certs",
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
			Type: corev1.SecretTypeOpaque,
		}

		dashboardSecretCSRF = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "kubernetes-dashboard-csrf",
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardSecretKeyHolder = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "kubernetes-dashboard-key-holder",
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "kubernetes-dashboard-settings",
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardDeploymentDashboard = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        v1beta1constants.DeploymentNameKubernetesDashboard,
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardDeploymentMetricsScraper = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        v1beta1constants.DeploymentNameDashboardMetricsScraper,
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardServiceDashboard = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardName,
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardServiceMetricsScraper = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "dashboard-metrics-scraper",
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		dashboardVpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:        kubernetesDashboardName,
				Namespace:   kubernetesDashboardNamespace,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress-controller",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxServiceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "nginx-ingress",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxServiceController = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress-controller",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxServiceBackend = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress-nginx-ingress-k8s-backend",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxRole = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxRoleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxClusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress",
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress",
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxDeploymentController = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress-controller",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxDeploymentBackend = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress-nginx-ingress-k8s-backend",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxIngressClass = &networkingv1.IngressClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:        v1beta1constants.ShootNginxIngressClass,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxRoleBindingPSP = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "gardener.cloud:psp:addons-nginx-ingress",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxPodDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "nginx-ingress-controller",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxVpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "addons-nginx-ingress-controller",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}

		nginxNetworkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "gardener.cloud--allow-to-from-nginx",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Mode: resourcesv1alpha1.ModeIgnore},
			},
		}
	)

	return registry.AddAllAndSerialize(
		dashboardNamespace,
		dashboardRole,
		dashboardRoleBinding,
		dashboardClusterRole,
		dashboardClusterRoleBinding,
		dashboardServiceAccount,
		dashboardSecretCerts,
		dashboardSecretCSRF,
		dashboardSecretKeyHolder,
		dashboardConfigMap,
		dashboardDeploymentDashboard,
		dashboardDeploymentMetricsScraper,
		dashboardServiceDashboard,
		dashboardServiceMetricsScraper,
		dashboardVpa,
		nginxConfigMap,
		nginxServiceAccount,
		nginxServiceController,
		nginxServiceBackend,
		nginxRole,
		nginxRoleBinding,
		nginxClusterRole,
		nginxClusterRoleBinding,
		nginxDeploymentController,
		nginxDeploymentBackend,
		nginxIngressClass,
		nginxRoleBindingPSP,
		nginxPodDisruptionBudget,
		nginxVpa,
		nginxNetworkPolicy,
	)
}
