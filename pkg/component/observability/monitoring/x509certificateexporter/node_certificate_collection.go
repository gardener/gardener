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
)

func (x *x509CertificateExporter) daemonSetList(resNamePrefix string, sa *corev1.ServiceAccount) []client.Object {
	daemonSets := make([]client.Object, 0, len(x.conf.workerGroups))

	for _, wg := range x.conf.workerGroups {
		var name string
		if wg.NameSuffix == "" {
			name = resNamePrefix
		} else {
			name = fmt.Sprintf("%s-%s", resNamePrefix, wg.NameSuffix)
		}
		daemonSets = append(daemonSets, x.daemonSet(name, sa, wg))
	}
	return daemonSets
}

func (x *x509CertificateExporter) daemonSet(
	resName string, sa *corev1.ServiceAccount,
	wg workerGroup,
) *appsv1.DaemonSet {
	var (
		labelz       = x.getGenericLabels(nodeCertificateLabelValue)
		args         = wg.GetArgs()
		volumeMounts []corev1.VolumeMount
		volumes      []corev1.Volume
		podSpec      corev1.PodSpec
	)

	for mountName, mount := range wg.Mounts {
		vol, volmount := getPathSetup(mount.Path, mountName)
		volumes = append(volumes, vol)
		volumeMounts = append(volumeMounts, volmount)
	}

	args = append(args, []string{fmt.Sprintf("--listen-address=:%d", port)}...)
	sort.Strings(args)
	podSpec = x.defaultPodSpec(sa)
	podSpec.Containers[0].Args = args
	podSpec.Volumes = volumes
	podSpec.Containers[0].VolumeMounts = volumeMounts
	podSpec.Containers[0].SecurityContext.AllowPrivilegeEscalation = ptr.To(true)
	podSpec.NodeSelector = wg.NodeSelector
	podSpec.Tolerations = wg.Tolerations
	podSpec.TopologySpreadConstraints = wg.TopologySpreadConstraints
	podSpec.Affinity = wg.Affinity

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: x.namespace,
			Labels:    labelz,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: wg.Selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelz,
				},
				Spec: podSpec,
			},
		},
	}
}

func (x *x509CertificateExporter) getHostCertificateMonitoringResources() []client.Object {
	var (
		resName    = nodeManagedResourceName + x.values.NameSuffix
		sa         = x.serviceAccount(resName)
		service    = x.service(resName, x.getGenericLabels(nodeCertificateLabelValue))
		sm         = x.serviceMonitor(resName, x.getGenericLabels(nodeCertificateLabelValue))
		daemonSets = x.daemonSetList(resName, sa)
	)

	return append(daemonSets, sa, service, sm)
}
