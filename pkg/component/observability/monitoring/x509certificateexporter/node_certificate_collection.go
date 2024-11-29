// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (x *x509CertificateExporter) daemonSet(
	resName string, sa *corev1.ServiceAccount,
) (*appsv1.DaemonSet, error) {
	if len(x.values.HostCertificates) == 0 {
		return nil, errors.New("no host certificates provided")
	}

	var (
		hostPathType = corev1.HostPathDirectory
		labelz       = x.getGenericLabels(nodeCertificateLabelValue)
		hostPaths    []string
		args         []string
		volumeMounts []corev1.VolumeMount
		volumes      []corev1.Volume
		podSpec      corev1.PodSpec
	)

	hostPaths, args = func() ([]string, []string) {
		paths := []string{}
		certArgs := []string{}
		for _, hc := range x.values.HostCertificates {
			paths = append(paths, hc.MountPath)
			certArgs = append(certArgs, hc.AsArgs()...)
		}
		return paths, certArgs
	}()

	args = append(args, func() []string {
		return []string{
			"--expose-relative-metrics",
			"--watch-kube-secrets",
			"--expose-per-cert-error-metrics",
			fmt.Sprintf("--listen-address=:%d", port),
		}
	}()...,
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
	}, nil
}

func (x *x509CertificateExporter) getHostCertificateMonitoringResources() ([]client.Object, error) {
	var (
		resName = nodeManagedResourceName + x.values.NameSuffix
		sa      = x.serviceAccount(resName)
		service = x.service(resName, x.getGenericLabels(nodeCertificateLabelValue))
		sm      = x.serviceMonitor(resName, x.getGenericLabels(nodeCertificateLabelValue))
		ds      *appsv1.DaemonSet
	)
	ds, err := x.daemonSet(resName, sa)

	if err != nil {
		return nil, fmt.Errorf("failed to create DaemonSet: %w", err)
	}

	return []client.Object{sa, service, sm, ds}, nil
}
