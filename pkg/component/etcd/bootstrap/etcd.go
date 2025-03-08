// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/controllerutils"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	metricsPort = 2381

	volumeNameData          = "main-etcd"
	volumeNameETCDCA        = "etcd-ca"
	volumeNameServerTLS     = "etcd-server-tls"
	volumeNameClientTLS     = "etcd-client-tls"
	volumeNamePeerCA        = "etcd-peer-ca"
	volumeNamePeerServerTLS = "etcd-peer-server-tls"

	volumeMountPathData          = "/var/etcd/data"
	volumeMountPathETCDCA        = "/var/etcd/ssl/ca"
	volumeMountPathServerTLS     = "/var/etcd/ssl/server"
	volumeMountPathPeerCA        = "/var/etcd/ssl/peer/ca"
	volumeMountPathPeerServerTLS = "/var/etcd/ssl/peer/server"
)

// Values is a set of configuration values for the Etcd component.
type Values struct {
	// Image is the container image used for Etcd.
	Image string
}

// New creates a new instance of DeployWaiter for the Etcd.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.Deployer {
	return &etcdDeployer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type etcdDeployer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (e *etcdDeployer) Deploy(ctx context.Context) error {
	etcdCASecret, serverSecret, clientSecret, err := etcd.GenerateClientServerCertificates(
		ctx,
		e.secretsManager,
		"main",
		[]string{"localhost"},
		[]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	)
	if err != nil {
		return fmt.Errorf("failed to generate etcd client/server certificates: %w", err)
	}

	etcdPeerCASecretName, peerServerSecretName, err := etcd.GeneratePeerCertificates(
		ctx,
		e.secretsManager,
		"main",
		[]string{"localhost"},
		[]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	)
	if err != nil {
		return fmt.Errorf("failed to generate etcd peer certificates: %w", err)
	}

	pod := e.emptyPod()

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, e.client, pod, func() error {
		pod.Labels = map[string]string{
			v1beta1constants.LabelApp:   "etcd",
			v1beta1constants.LabelRole:  "main",
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		}
		pod.Spec = corev1.PodSpec{
			Containers: []corev1.Container{{
				Command: []string{
					"etcd",
					"--advertise-client-urls=https://127.0.0.1:2379,https://[::1]:2379",
					"--cert-file=" + volumeMountPathServerTLS + "/tls.crt",
					"--client-cert-auth=true",
					"--data-dir=" + volumeMountPathData,
					"--experimental-initial-corrupt-check=true",
					"--experimental-watch-progress-notify-interval=5s",
					"--initial-advertise-peer-urls=https://127.0.0.1:2380,https://[::1]:2380",
					"--initial-cluster=etcd-main=https://127.0.0.1:2380",
					"--key-file=" + volumeNamePeerServerTLS + "/tls.key",
					"--listen-client-urls=https://127.0.0.1:2379,https://[::1]:2379",
					"--listen-metrics-urls=http://127.0.0.1:2381,http://[::1]:2381",
					"--listen-peer-urls=https://127.0.0.1:2380,https://[::1]:2380",
					"--name=etcd-main",
					"--peer-cert-file=" + volumeNamePeerServerTLS + "/tls.crt",
					"--peer-client-cert-auth=true",
					"--peer-key-file=" + volumeNamePeerServerTLS + "/tls.key",
					"--peer-trusted-ca-file=" + volumeMountPathPeerCA + "/bundle.crt",
					"--snapshot-count=10000",
					"--trusted-ca-file=" + volumeNameETCDCA + "/bundle.crt",
				},
				Image:           e.values.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/livez",
							Scheme: corev1.URISchemeHTTP,
							Port:   intstr.FromInt32(metricsPort),
						},
					},
					SuccessThreshold:    1,
					FailureThreshold:    8,
					InitialDelaySeconds: 10,
					PeriodSeconds:       10,
					TimeoutSeconds:      15,
				},
				Name: "etcd",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				StartupProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/health",
							Scheme: corev1.URISchemeHTTP,
							Port:   intstr.FromInt32(metricsPort),
						},
					},
					SuccessThreshold:    1,
					FailureThreshold:    24,
					InitialDelaySeconds: 10,
					PeriodSeconds:       10,
					TimeoutSeconds:      15,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						MountPath: volumeMountPathData,
						Name:      volumeNameData,
					},
					{
						MountPath: volumeMountPathETCDCA,
						Name:      volumeNameETCDCA,
					},
					{
						MountPath: volumeMountPathServerTLS,
						Name:      volumeNameServerTLS,
					},
					{
						MountPath: "/var/etcd/ssl/client",
						Name:      volumeNameClientTLS,
					},
					{
						MountPath: volumeMountPathPeerCA,
						Name:      volumeNamePeerCA,
					},
					{
						MountPath: volumeMountPathPeerServerTLS,
						Name:      volumeNamePeerServerTLS,
					},
				},
			}},
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: volumeNameData,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/etcd",
							Type: ptr.To(corev1.HostPathDirectoryOrCreate),
						},
					},
				},
				{
					Name: volumeNameETCDCA,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: etcdCASecret.Name,
						},
					},
				},
				{
					Name: volumeNamePeerCA,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: etcdPeerCASecretName,
						},
					},
				},
				{
					Name: volumeNameServerTLS,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: serverSecret.Name,
						},
					},
				},
				{
					Name: volumeNameClientTLS,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: clientSecret.Name,
						},
					},
				},
				{
					Name: volumeNamePeerServerTLS,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: peerServerSecretName,
						},
					},
				},
			},
		}

		return nil
	})

	return err
}

func (e *etcdDeployer) Destroy(_ context.Context) error {
	return nil
}

func (e *etcdDeployer) emptyPod() *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main", Namespace: e.namespace}}
}
