// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"net"

	appsv1 "k8s.io/api/apps/v1"
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
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	volumeNameData          = "data"
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
	// Role is the role of this etcd instance (main or events).
	Role string
	// PortClient is the port for the client connections.
	PortClient int32
	// PortPeer is the port for the peer connections.
	PortPeer int32
	// PortMetrics is the port for the metrics connections.
	PortMetrics int32
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
		e.values.Role,
		[]string{"localhost"},
		[]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	)
	if err != nil {
		return fmt.Errorf("failed to generate etcd client/server certificates: %w", err)
	}

	etcdPeerCASecretName, peerServerSecretName, err := etcd.GeneratePeerCertificates(
		ctx,
		e.secretsManager,
		e.values.Role,
		[]string{"localhost"},
		[]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	)
	if err != nil {
		return fmt.Errorf("failed to generate etcd peer certificates: %w", err)
	}

	statefulSet := e.emptyStatefulSet()
	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, e.client, statefulSet, func() error {
		statefulSet.Labels = e.labels()
		statefulSet.Spec = appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: e.labels(),
			},
			Replicas: ptr.To[int32](0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: e.labels()},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Command: []string{
							"etcd",
							"--name=" + statefulSet.Name,
							"--data-dir=" + volumeMountPathData,
							"--experimental-initial-corrupt-check=true",
							"--experimental-watch-progress-notify-interval=5s",
							"--snapshot-count=10000",
							fmt.Sprintf("--advertise-client-urls=https://localhost:%d", e.values.PortClient),
							fmt.Sprintf("--initial-advertise-peer-urls=https://localhost:%d", e.values.PortPeer),
							fmt.Sprintf("--listen-client-urls=https://localhost:%d", e.values.PortClient),
							fmt.Sprintf("--initial-cluster=%s=https://localhost:%d", statefulSet.Name, e.values.PortPeer),
							fmt.Sprintf("--listen-peer-urls=https://localhost:%d", e.values.PortPeer),
							fmt.Sprintf("--listen-metrics-urls=http://localhost:%d", e.values.PortMetrics),
							"--client-cert-auth=true",
							fmt.Sprintf("--trusted-ca-file=%s/%s", volumeMountPathETCDCA, secretsutils.DataKeyCertificateBundle),
							fmt.Sprintf("--cert-file=%s/%s", volumeMountPathServerTLS, secretsutils.DataKeyCertificate),
							fmt.Sprintf("--key-file=%s/%s", volumeMountPathServerTLS, secretsutils.DataKeyPrivateKey),
							"--peer-client-cert-auth=true",
							fmt.Sprintf("--peer-trusted-ca-file=%s/%s", volumeMountPathPeerCA, secretsutils.DataKeyCertificateBundle),
							fmt.Sprintf("--peer-cert-file=%s/%s", volumeMountPathPeerServerTLS, secretsutils.DataKeyCertificate),
							fmt.Sprintf("--peer-key-file=%s/%s", volumeMountPathPeerServerTLS, secretsutils.DataKeyPrivateKey),
						},
						Image:           e.values.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Host:   "localhost",
									Path:   "/livez",
									Scheme: corev1.URISchemeHTTP,
									Port:   intstr.FromInt32(e.values.PortMetrics),
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
									Host:   "localhost",
									Path:   "/health",
									Scheme: corev1.URISchemeHTTP,
									Port:   intstr.FromInt32(e.values.PortMetrics),
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
									Path: "/var/lib/" + statefulSet.Name + "/data",
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

func (e *etcdDeployer) emptyStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "etcd-" + e.values.Role + "-0", Namespace: e.namespace}}
}

func (e *etcdDeployer) labels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:   "etcd",
		v1beta1constants.LabelRole:  e.values.Role,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
	}
}
