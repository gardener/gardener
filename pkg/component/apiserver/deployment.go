// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	volumeNameCAEtcd          = "ca-etcd"
	volumeNameEtcdClient      = "etcd-client"
	volumeNameServer          = "server"
	volumeMountPathCAEtcd     = "/srv/kubernetes/etcd/ca"
	volumeMountPathEtcdClient = "/srv/kubernetes/etcd/client"
	volumeMountPathServer     = "/srv/kubernetes/apiserver"
)

// InjectDefaultSettings injects default settings into `gardener-apiserver` and `kube-apiserver` deployments.
func InjectDefaultSettings(
	deployment *appsv1.Deployment,
	namePrefix string,
	values Values,
	secretCAETCD *corev1.Secret,
	secretETCDClient *corev1.Secret,
	secretServer *corev1.Secret,
) {
	etcdAddress := namePrefix + etcdconstants.ServiceName(v1beta1constants.ETCDRoleMain)
	if values.RunsAsStaticPod {
		etcdAddress = "localhost"
	}

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args,
		"--http2-max-streams-per-connection=1000",
		fmt.Sprintf("--etcd-cafile=%s/%s", volumeMountPathCAEtcd, secrets.DataKeyCertificateBundle),
		fmt.Sprintf("--etcd-certfile=%s/%s", volumeMountPathEtcdClient, secrets.DataKeyCertificate),
		fmt.Sprintf("--etcd-keyfile=%s/%s", volumeMountPathEtcdClient, secrets.DataKeyPrivateKey),
		fmt.Sprintf("--etcd-servers=https://%s:%d", etcdAddress, etcdconstants.PortEtcdClient),
		"--livez-grace-period=1m",
		"--profiling=false",
		"--shutdown-delay-duration=15s",
		fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificate),
		fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.DataKeyPrivateKey),
		"--tls-cipher-suites="+strings.Join(kubernetesutils.TLSCipherSuites, ","),
	)

	if values.FeatureGates != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, kubernetesutils.FeatureGatesToCommandLineParameter(values.FeatureGates))
	}

	if values.Requests != nil {
		if values.Requests.MaxNonMutatingInflight != nil {
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--max-requests-inflight=%d", *values.Requests.MaxNonMutatingInflight))
		}
		if values.Requests.MaxMutatingInflight != nil {
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--max-mutating-requests-inflight=%d", *values.Requests.MaxMutatingInflight))
		}
	}

	if values.Logging != nil {
		if values.Logging.HTTPAccessVerbosity != nil {
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--vmodule=httplog=%d", *values.Logging.HTTPAccessVerbosity))
		}
		if values.Logging.Verbosity != nil {
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--v=%d", *values.Logging.Verbosity))
		}
	}

	if values.WatchCacheSizes != nil && len(values.WatchCacheSizes.Resources) > 0 {
		if values.WatchCacheSizes != nil && values.WatchCacheSizes.Default != nil {
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--default-watch-cache-size=%d", *values.WatchCacheSizes.Default))
		}

		var sizes []string
		for _, resource := range values.WatchCacheSizes.Resources {
			size := resource.Resource
			if resource.APIGroup != nil {
				size += "." + *resource.APIGroup
			}
			size += fmt.Sprintf("#%d", resource.CacheSize)

			sizes = append(sizes, size)
		}

		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--watch-cache-sizes="+strings.Join(sizes, ","))
	}

	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      volumeNameCAEtcd,
			MountPath: volumeMountPathCAEtcd,
		},
		corev1.VolumeMount{
			Name:      volumeNameEtcdClient,
			MountPath: volumeMountPathEtcdClient,
		},
		corev1.VolumeMount{
			Name:      volumeNameServer,
			MountPath: volumeMountPathServer,
		},
	)

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: volumeNameCAEtcd,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretCAETCD.Name,
				},
			},
		},
		corev1.Volume{
			Name: volumeNameEtcdClient,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretETCDClient.Name,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		},
		corev1.Volume{
			Name: volumeNameServer,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretServer.Name,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		},
	)
}
