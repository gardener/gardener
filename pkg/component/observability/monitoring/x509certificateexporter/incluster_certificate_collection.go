// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

func (x *x509CertificateExporter) deployment(
	resName string, sa *corev1.ServiceAccount,
) *appsv1.Deployment {

	var (
		podLabels = x.getGenericLabels(inClusterCertificateLabelValue)
		args      []string
		podSpec   corev1.PodSpec
	)

	podLabels[v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer] = v1beta1constants.LabelNetworkPolicyAllowed
	args = func() []string {
		defaultArgs := []string{
			"--expose-relative-metrics",
			"--expose-per-cert-error-metrics",
			"--watch-kube-secrets",
			"--max-cache-duration=" + x.conf.inCluster.MaxCacheDuration.String(),
			fmt.Sprintf("--listen-address=:%d", port),
		}
		secretTypes := secretTypesAsArgs(x.conf.inCluster.SecretTypes)
		configMapKeys := configMapKeysAsArgs(x.conf.inCluster.ConfigMapKeys)
		includedLabels := includedLabelsAsArgs(x.conf.inCluster.IncludeLabels)
		excludedLabels := excludedLabelsAsArgs(x.conf.inCluster.ExcludeLabels)
		excludedNamespaces := excludedNamespacesAsArgs(x.conf.inCluster.ExcludeNamespaces)
		includedNamespaces := includedNamespacesAsArgs(x.conf.inCluster.IncludeNamespaces)
		args := make([]string, 0, len(secretTypes)+len(configMapKeys)+len(includedLabels)+len(excludedLabels)+len(excludedNamespaces)+len(includedNamespaces)+len(defaultArgs))
		args = append(args, secretTypes...)
		args = append(args, configMapKeys...)
		args = append(args, includedLabels...)
		args = append(args, excludedLabels...)
		args = append(args, excludedNamespaces...)
		args = append(args, includedNamespaces...)
		args = append(args, defaultArgs...)
		sort.Strings(args)
		return args
	}()
	podSpec = x.defaultPodSpec(sa)
	podSpec.Containers[0].Args = args

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: x.namespace,
			Labels:    x.getGenericLabels(inClusterCertificateLabelValue),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(*x.conf.inCluster.Replicas)),
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: podSpec,
			},
		},
	}
}

func (x *x509CertificateExporter) getInClusterCertificateMonitoringResources() []client.Object {
	var (
		resName = inClusterManagedResourceName + x.values.NameSuffix
		sa      = x.serviceAccount(resName)
		cr      = x.inClusterClusterRole(clusterRoleName, x.values)
		crb     = x.inClusterClusterRoleBinding(clusterRoleBindingName, sa, cr)
		service = x.service(resName, x.getGenericLabels(inClusterCertificateLabelValue))
		sm      = x.serviceMonitor(resName, x.getGenericLabels(inClusterCertificateLabelValue))
		dep     = x.deployment(resName, sa)
	)
	return []client.Object{sa, cr, crb, service, sm, dep}
}
