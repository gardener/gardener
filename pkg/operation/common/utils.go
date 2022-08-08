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
	"fmt"
	"math/big"
	"net"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FilterEntriesByPrefix returns a list of strings which begin with the given prefix.
func FilterEntriesByPrefix(prefix string, entries []string) []string {
	var result []string
	for _, entry := range entries {
		if strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
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

// DeleteLoki  deletes all resources of the Loki in a given namespace.
func DeleteLoki(ctx context.Context, k8sClient client.Client, namespace string) error {
	resources := []client.Object{
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: namespace}},
		&hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: namespace}},
	}

	return kutil.DeleteObjects(ctx, k8sClient, resources...)
}

// DeleteSeedLoggingStack deletes all seed resource of the logging stack in the garden namespace.
func DeleteSeedLoggingStack(ctx context.Context, k8sClient client.Client) error {
	resources := []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-config", Namespace: v1beta1constants.GardenNamespace}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-fluentbit", Namespace: v1beta1constants.GardenNamespace}},
		&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit"}},
		&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "loki"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
	}

	if err := kutil.DeleteObjects(ctx, k8sClient, resources...); err != nil {
		return err
	}

	return DeleteLoki(ctx, k8sClient, v1beta1constants.GardenNamespace)
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
		&networkingv1.Ingress{
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
		// TODO(rfranzke): Remove this secret in a future release.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-tls",
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

	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"component": "grafana",
			"role":      role,
		},
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &appsv1.Deployment{}, append(deleteOptions, client.PropagationPolicy(metav1.DeletePropagationForeground))...); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &corev1.ConfigMap{}, deleteOptions...); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &networkingv1.Ingress{}, deleteOptions...); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &corev1.Secret{}, deleteOptions...); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := k8sClient.Client().Delete(
		ctx,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "grafana-" + role,
				Namespace: namespace,
			}},
	); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}
