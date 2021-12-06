// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const annotationSeedIngressClass = "seed.gardener.cloud/ingress-class"

func migrateIngressClassForShootIngresses(ctx context.Context, gardenClient, seedClient client.Client, seed *Seed, newClass string, kubernetesVersion *semver.Version) error {
	if oldClass, ok := seed.GetInfo().Annotations[annotationSeedIngressClass]; ok && oldClass == newClass {
		return nil
	}

	shootNamespaces := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, shootNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return err
	}

	if err := switchIngressClass(ctx, seedClient, kutil.Key(v1beta1constants.GardenNamespace, "aggregate-prometheus"), newClass, kubernetesVersion); err != nil {
		return err
	}
	if err := switchIngressClass(ctx, seedClient, kutil.Key(v1beta1constants.GardenNamespace, "grafana"), newClass, kubernetesVersion); err != nil {
		return err
	}

	for _, ns := range shootNamespaces.Items {
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "alertmanager"), newClass, kubernetesVersion); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "prometheus"), newClass, kubernetesVersion); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "grafana-operators"), newClass, kubernetesVersion); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "grafana-users"), newClass, kubernetesVersion); err != nil {
			return err
		}
	}

	return seed.UpdateInfo(ctx, gardenClient, false, func(seed *gardencorev1beta1.Seed) error {
		metav1.SetMetaDataAnnotation(&seed.ObjectMeta, annotationSeedIngressClass, newClass)
		return nil
	})
}

func switchIngressClass(ctx context.Context, seedClient client.Client, ingressKey types.NamespacedName, newClass string, kubernetesVersion *semver.Version) error {
	// We need to use `versionutils.CompareVersions` because this function normalizes the seed version first.
	// This is especially necessary if the seed cluster is a non Gardener managed cluster and thus might have some
	// custom version suffix.
	lessEqual121, err := versionutils.CompareVersions(kubernetesVersion.String(), "<=", "1.21.x")
	if err != nil {
		return err
	}
	if lessEqual121 {
		ingress := &extensionsv1beta1.Ingress{}

		if err := seedClient.Get(ctx, ingressKey, ingress); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		annotations := ingress.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[networkingv1beta1.AnnotationIngressClass] = newClass
		ingress.SetAnnotations(annotations)

		return seedClient.Update(ctx, ingress)
	}

	ingress := &networkingv1.Ingress{}

	if err := seedClient.Get(ctx, ingressKey, ingress); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	ingress.Spec.IngressClassName = &newClass
	delete(ingress.Annotations, networkingv1beta1.AnnotationIngressClass)

	return seedClient.Update(ctx, ingress)
}

func computeNginxIngress(seed *Seed) map[string]interface{} {
	values := map[string]interface{}{
		"enabled": managedIngress(seed),
	}

	if seed.GetInfo().Spec.Ingress != nil && seed.GetInfo().Spec.Ingress.Controller.ProviderConfig != nil {
		values["config"] = seed.GetInfo().Spec.Ingress.Controller.ProviderConfig
	}

	return values
}

func computeNginxIngressClass(seed *Seed, kubernetesVersion *semver.Version) (string, error) {
	managed := managedIngress(seed)

	// We need to use `versionutils.CompareVersions` because this function normalizes the seed version first.
	// This is especially necessary if the seed cluster is a non Gardener managed cluster and thus might have some
	// custom version suffix.
	greaterEqual122, err := versionutils.CompareVersions(kubernetesVersion.String(), ">=", "1.22")
	if err != nil {
		return "", err
	}

	if managed && greaterEqual122 {
		return v1beta1constants.SeedNginxIngressClass122, nil
	}
	if managed {
		return v1beta1constants.SeedNginxIngressClass, nil
	}
	return v1beta1constants.ShootNginxIngressClass, nil
}

func deleteIngressController(ctx context.Context, c client.Client) error {
	return kutil.DeleteObjects(
		ctx,
		c,
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:seed:nginx-ingress"}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:seed:nginx-ingress"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-k8s-backend", Namespace: v1beta1constants.GardenNamespace}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress", Namespace: v1beta1constants.GardenNamespace}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-k8s-backend", Namespace: v1beta1constants.GardenNamespace}},
	)
}
