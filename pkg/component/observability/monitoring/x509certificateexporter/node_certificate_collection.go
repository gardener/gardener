// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (x *x509CertificateExporter) daemonSetList(resNamePrefix string, sa *corev1.ServiceAccount) []client.Object {
	daemonSets := make([]client.Object, 0, len(x.conf.WorkerGroups))

	for _, wg := range x.conf.WorkerGroups {
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
		labelz           = x.getGenericLabels(nodeCertificateLabelValue)
		args             = wg.GetArgs()
		volumeMounts     []corev1.VolumeMount
		volumes          []corev1.Volume
		podSpec          corev1.PodSpec
		defaultMountPath = func(hostPath, mountPath string) string {
			if mountPath == "" {
				return hostPath
			}
			return mountPath
		}
	)

	labelz["part-of"] = resName

	for mountName, mount := range wg.Mounts {
		vol, volmount := getPathSetup(mount.HostPath, defaultMountPath(mount.HostPath, mount.MountPath), mountName)
		volumes = append(volumes, vol)
		volumeMounts = append(volumeMounts, volmount)
	}

	podSpec = x.defaultPodSpec(sa, wg.NodeSelector)
	podSpec.Containers[0].Args = args
	podSpec.Volumes = volumes
	podSpec.Containers[0].VolumeMounts = volumeMounts
	podSpec.Containers[0].SecurityContext.AllowPrivilegeEscalation = ptr.To(true)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: x.namespace,
			Labels:    labelz,
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelz,
				},
				Spec: podSpec,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labelz,
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
