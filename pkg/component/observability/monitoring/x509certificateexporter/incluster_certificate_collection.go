// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"errors"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

func (x *x509CertificateExporter) deployment(
	resName string, sa *corev1.ServiceAccount,
) (*appsv1.Deployment, error) {
	if len(x.values.SecretTypes) == 0 {
		return nil, errors.New("no secret types provided")
	}

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
			"--max-cache-duration=" + x.values.CacheDuration.Duration.String(),
			fmt.Sprintf("--listen-address=:%d", port),
		}
		secretTypes := x.values.SecretTypes.AsArgs()
		configMapKeys := x.values.ConfigMapKeys.AsArgs()
		includedLabels := x.values.IncludeLabels.AsArgs()
		excludedLabels := x.values.ExcludeLabels.AsArgs()
		excludedNamespaces := x.values.ExcludeNamespaces.AsArgs()
		includedNamespaces := x.values.IncludeNamespaces.AsArgs()
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
			Replicas: &x.values.Replicas,
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
	}, nil
}

func (x *x509CertificateExporter) getInClusterCertificateMonitoringResources() ([]client.Object, error) {
	var (
		resName = inClusterManagedResourceName + x.values.NameSuffix
		sa      = x.serviceAccount(resName)
		cr      = x.inClusterClusterRole(clusterRoleName, x.values)
		crb     = x.inClusterClusterRoleBinding(clusterRoleBindingName, sa, cr)
		service = x.service(resName, x.getGenericLabels(inClusterCertificateLabelValue))
		sm      = x.serviceMonitor(resName, x.getGenericLabels(inClusterCertificateLabelValue))
		dep     *appsv1.Deployment
	)
	dep, err := x.deployment(resName, sa)

	if err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}
	return []client.Object{sa, cr, crb, service, sm, dep}, nil
}
