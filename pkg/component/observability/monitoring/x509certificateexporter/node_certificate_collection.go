// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"crypto/sha256"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (x *x509CertificateExporter) daemonSetList(resNamePrefix string, sa *corev1.ServiceAccount) ([]client.Object, error) {
	if len(x.values.WorkerGroups) == 0 {
		return nil, nil
	}
	daemonSets := make([]client.Object, 0, len(x.values.WorkerGroups))

	for name, group := range x.values.WorkerGroups {
		if len(group.MountPaths) == 0 {
			return nil, fmt.Errorf("no mount paths provided for worker group %s", name)
		}
		if len(group.CertificatePaths) == 0 {
			return nil, fmt.Errorf("no certificate paths provided for worker group %s", name)
		}
		certificatesToMonitor, certificateDirsToMonitor, err := getPathArgs(group.CertificatePaths)
		if err != nil {
			return nil, fmt.Errorf("failed to create certificate path args for worker group %s: %w", name, err)
		}
		daemonSets = append(daemonSets, x.daemonSet(fmt.Sprintf("%s-%s", resNamePrefix, name), sa, certificatesToMonitor, certificateDirsToMonitor, group.Selector))
	}

	return daemonSets, nil
}

func (x *x509CertificateExporter) daemonSet(
	resName string, sa *corev1.ServiceAccount,
	hostPaths []string, certArgs []string,
	selector *metav1.LabelSelector,
) *appsv1.DaemonSet {
	var (
		hostPathType = corev1.HostPathDirectory
		labelz       = x.getGenericLabels(nodeCertificateLabelValue)
		args         = certArgs
		volumeMounts []corev1.VolumeMount
		volumes      []corev1.Volume
		podSpec      corev1.PodSpec
	)

	args = append(args,
		"--expose-relative-metrics",
		"--expose-per-cert-error-metrics",
		fmt.Sprintf("--listen-address=:%d", port),
	)
	sort.Strings(args)
	sort.Strings(hostPaths)

	volumeMounts = make([]corev1.VolumeMount, len(hostPaths))
	volumes = make([]corev1.Volume, len(hostPaths))

	for idx, path := range hostPaths {
		name := fmt.Sprintf("%x", sha256.Sum256([]byte(path)))
		volumes[idx] = corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: path,
					Type: &hostPathType,
				},
			},
		}
		volumeMounts[idx] = corev1.VolumeMount{
			Name:      name,
			ReadOnly:  true,
			MountPath: path,
		}
	}
	podSpec = x.defaultPodSpec(sa)
	podSpec.Containers[0].Args = args
	podSpec.Volumes = volumes
	podSpec.Containers[0].VolumeMounts = volumeMounts
	podSpec.Containers[0].SecurityContext.AllowPrivilegeEscalation = ptr.To(true)
	if selector != nil {
		podSpec.NodeSelector = selector.MatchLabels
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: x.namespace,
			Labels:    labelz,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelz,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelz,
				},
				Spec: podSpec,
			},
		},
	}
}

func (x *x509CertificateExporter) getHostCertificateMonitoringResources() ([]client.Object, error) {
	var (
		resName = nodeManagedResourceName + x.values.NameSuffix
		sa      = x.serviceAccount(resName)
		service = x.service(resName, x.getGenericLabels(nodeCertificateLabelValue))
		sm      = x.serviceMonitor(resName, x.getGenericLabels(nodeCertificateLabelValue))
	)
	objList, err := x.daemonSetList(resName, sa)

	if err != nil {
		return nil, fmt.Errorf("failed to create DaemonSets: %w", err)
	}

	if objList == nil {
		return nil, nil
	}

	objList = append(objList, sa, service, sm)

	return objList, nil
}
