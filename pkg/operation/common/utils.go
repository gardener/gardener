// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSecretKeysWithPrefix returns a list of keys of the given map <m> which are prefixed with <kind>.
func GetSecretKeysWithPrefix(kind string, m map[string]*corev1.Secret) []string {
	var result []string
	for key := range m {
		if strings.HasPrefix(key, kind) {
			result = append(result, key)
		}
	}
	return result
}

// ComputeOffsetIP parses the provided <subnet> and offsets with the value of <offset>.
// For example, <subnet> = 100.64.0.0/11 and <offset> = 10 the result would be 100.64.0.10
// IPv6 and IPv4 is supported.
func ComputeOffsetIP(subnet *net.IPNet, offset int64) (net.IP, error) {
	if subnet == nil {
		return nil, fmt.Errorf("subnet is nil")
	}

	isIPv6 := false

	bytes := subnet.IP.To4()
	if bytes == nil {
		isIPv6 = true
		bytes = subnet.IP.To16()
	}

	ip := net.IP(big.NewInt(0).Add(big.NewInt(0).SetBytes(bytes), big.NewInt(offset)).Bytes())

	if !subnet.Contains(ip) {
		return nil, fmt.Errorf("cannot compute IP with offset %d - subnet %q too small", offset, subnet)
	}

	// there is no broadcast address on IPv6
	if isIPv6 {
		return ip, nil
	}

	for i := range ip {
		// IP address is not the same, so it's not the broadcast ip.
		if ip[i] != ip[i]|^subnet.Mask[i] {
			return ip.To4(), nil
		}
	}

	return nil, fmt.Errorf("computed IPv4 address %q is broadcast for subnet %q", ip, subnet)
}

// GenerateAddonConfig returns the provided <values> in case <enabled> is true. Otherwise, nil is
// being returned.
func GenerateAddonConfig(values map[string]interface{}, enabled bool) map[string]interface{} {
	v := map[string]interface{}{
		"enabled": enabled,
	}
	if enabled {
		for key, value := range values {
			v[key] = value
		}
	}
	return v
}

// DeleteHvpa delete all resources required for the HVPA in the given namespace.
func DeleteHvpa(ctx context.Context, k8sClient kubernetes.Interface, namespace string) error {
	if k8sClient == nil {
		return fmt.Errorf("require kubernetes client")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", v1beta1constants.GardenRole, v1beta1constants.GardenRoleHvpa),
	}

	// Delete all CRDs with label "gardener.cloud/role=hvpa"
	// Workaround: Due to https://github.com/gardener/gardener/issues/2257, we first list the HVPA CRDs and then remove
	// them one by one.
	crdList, err := k8sClient.APIExtension().ApiextensionsV1beta1().CustomResourceDefinitions().List(ctx, listOptions)
	if err != nil {
		return err
	}
	for _, crd := range crdList.Items {
		if err := k8sClient.APIExtension().ApiextensionsV1beta1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	// Delete all Deployments with label "gardener.cloud/role=hvpa"
	deletePropagation := metav1.DeletePropagationForeground
	if err := k8sClient.Kubernetes().AppsV1().Deployments(namespace).DeleteCollection(ctx, metav1.DeleteOptions{PropagationPolicy: &deletePropagation}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Delete all ClusterRoles with label "gardener.cloud/role=hvpa"
	if err := k8sClient.Kubernetes().RbacV1().ClusterRoles().DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Delete all ClusterRoleBindings with label "gardener.cloud/role=hvpa"
	if err := k8sClient.Kubernetes().RbacV1().ClusterRoleBindings().DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Delete all ServiceAccounts with label "gardener.cloud/role=hvpa"
	if err := k8sClient.Kubernetes().CoreV1().ServiceAccounts(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

// DeleteVpa delete all resources required for the VPA in the given namespace.
func DeleteVpa(ctx context.Context, c client.Client, namespace string, isShoot bool) error {
	resources := []client.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAAdmissionController, Namespace: namespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPARecommender, Namespace: namespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAUpdater, Namespace: namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "vpa-webhook", Namespace: namespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAAdmissionController, Namespace: namespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPARecommender, Namespace: namespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAUpdater, Namespace: namespace}},
	}

	if isShoot {
		resources = append(resources,
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-admission-controller", Namespace: namespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-recommender", Namespace: namespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: VPASecretName, Namespace: namespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-updater", Namespace: namespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-kube-apiserver-to-vpa-admission-controller", Namespace: namespace}},
		)
	} else {
		resources = append(resources,
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:actor"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:admission-controller"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:checkpoint-actor"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:metrics-reader"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:target-reader"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:actor"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:admission-controller"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:checkpoint-actor"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:metrics-reader"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:target-reader"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "vpa-admission-controller", Namespace: namespace}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "vpa-recommender", Namespace: namespace}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "vpa-updater", Namespace: namespace}},
			&admissionregistrationv1beta1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "vpa-webhook-config-seed"}},
		)
	}

	for _, resource := range resources {
		if err := c.Delete(ctx, resource); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

// DeleteShootLoggingStack deletes all shoot resource of the logging stack in the given namespace.
func DeleteShootLoggingStack(ctx context.Context, k8sClient client.Client, namespace string) error {
	return DeleteLoki(ctx, k8sClient, namespace)
}

// DeleteLoki  deletes all resources of the Loki in a given namespace.
func DeleteLoki(ctx context.Context, k8sClient client.Client, namespace string) error {
	resources := []client.Object{
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: namespace}},
		&hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: namespace}},
	}

	for _, resource := range resources {
		if err := k8sClient.Delete(ctx, resource); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
			return err
		}
	}
	return nil
}

// DeleteSeedLoggingStack deletes all seed resource of the logging stack in the garden namespace.
func DeleteSeedLoggingStack(ctx context.Context, k8sClient client.Client) error {
	resources := []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-config", Namespace: v1beta1constants.GardenNamespace}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-fluentbit", Namespace: v1beta1constants.GardenNamespace}},
		&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit"}},
		&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "loki"}},
		&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: GardenLokiPriorityClassName}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
	}

	for _, resource := range resources {
		if err := k8sClient.Delete(ctx, resource); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
			return err
		}
	}

	return DeleteLoki(ctx, k8sClient, v1beta1constants.GardenNamespace)
}

// DeleteReserveExcessCapacity deletes the deployment and priority class for excess capacity
func DeleteReserveExcessCapacity(ctx context.Context, k8sClient client.Client) error {
	if k8sClient == nil {
		return errors.New("must provide non-nil kubernetes client to common.DeleteReserveExcessCapacity")
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reserve-excess-capacity",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	if err := k8sClient.Delete(ctx, deploy); client.IgnoreNotFound(err) != nil {
		return err
	}

	priorityClass := &schedulingv1beta1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-reserve-excess-capacity",
		},
	}
	return client.IgnoreNotFound(k8sClient.Delete(ctx, priorityClass))
}

// DeleteAlertmanager deletes all resources of the Alertmanager in a given namespace.
func DeleteAlertmanager(ctx context.Context, k8sClient client.Client, namespace string) error {
	objs := []client.Object{
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.StatefulSetNameAlertManager,
				Namespace: namespace,
			},
		},
		&extensionsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager",
				Namespace: namespace,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-client",
				Namespace: namespace,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager",
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-basic-auth",
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      AlertManagerTLS,
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-config",
				Namespace: namespace,
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-db-alertmanager-0",
				Namespace: namespace,
			},
		},
	}

	return kutil.DeleteObjects(ctx, k8sClient, objs...)
}

// DeleteGrafanaByRole deletes the monitoring stack for the shoot owner.
func DeleteGrafanaByRole(ctx context.Context, k8sClient kubernetes.Interface, namespace, role string) error {
	if k8sClient == nil {
		return fmt.Errorf("require kubernetes client")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", "component", "grafana", "role", role),
	}

	deletePropagation := metav1.DeletePropagationForeground
	if err := k8sClient.Kubernetes().AppsV1().Deployments(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{
			PropagationPolicy: &deletePropagation,
		}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().CoreV1().ConfigMaps(namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().ExtensionsV1beta1().Ingresses(namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().CoreV1().Secrets(namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().CoreV1().Services(namespace).Delete(
		ctx, fmt.Sprintf("grafana-%s", role), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// ReadServiceAccountSigningKeySecret reads the signing key secret to extract the signing key.
// It errors if there is no value at ServiceAccountSigningKeySecretDataKey.
func ReadServiceAccountSigningKeySecret(secret *corev1.Secret) (string, error) {
	data, ok := secret.Data[ServiceAccountSigningKeySecretDataKey]
	if !ok {
		return "", fmt.Errorf("no signing key secret in secret %s/%s at .Data.%s", secret.Namespace, secret.Name, ServiceAccountSigningKeySecretDataKey)
	}

	return string(data), nil
}

// GetServiceAccountSigningKeySecret gets the signing key from the secret with the given name and namespace.
func GetServiceAccountSigningKeySecret(ctx context.Context, c client.Client, shootNamespace, secretName string) (string, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, kutil.Key(shootNamespace, secretName), secret); err != nil {
		return "", err
	}

	return ReadServiceAccountSigningKeySecret(secret)
}

// DeleteDeploymentsHavingDeprecatedRoleLabelKey deletes the Deployments with the passed object keys if
// the corresponding Deployment .spec.selector contains the deprecated "garden.sapcloud.io/role" label key.
func DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx context.Context, c client.Client, keys []client.ObjectKey) error {
	for _, key := range keys {
		deployment := &appsv1.Deployment{}
		if err := c.Get(ctx, key, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return err
		}

		if _, ok := deployment.Spec.Selector.MatchLabels[v1beta1constants.DeprecatedGardenRole]; ok {
			if err := c.Delete(ctx, deployment); client.IgnoreNotFound(err) != nil {
				return err
			}

			if err := kutil.WaitUntilResourceDeleted(ctx, c, deployment, 2*time.Second); err != nil {
				return err
			}
		}
	}

	return nil
}
