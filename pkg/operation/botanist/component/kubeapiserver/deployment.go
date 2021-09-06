// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/gardener/gardener-resource-manager/pkg/controller/garbagecollector/references"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
)

const (
	// SecretNameBasicAuth is the name of the secret containing basic authentication credentials for the kube-apiserver.
	SecretNameBasicAuth = "kube-apiserver-basic-auth"
	// SecretNameEtcdEncryption is the name of the secret which contains the EncryptionConfiguration. The
	// EncryptionConfiguration contains a key which the kube-apiserver uses for encrypting selected etcd content.
	SecretNameEtcdEncryption = "etcd-encryption-secret"
	// SecretNameKubeAggregator is the name of the secret for the kube-aggregator when talking to the kube-apiserver.
	SecretNameKubeAggregator = "kube-aggregator"
	// SecretNameKubeAPIServerToKubelet is the name of the secret for the kube-apiserver credentials when talking to
	// kubelets.
	SecretNameKubeAPIServerToKubelet = "kube-apiserver-kubelet"
	// SecretNameServer is the name of the secret for the kube-apiserver server certificates.
	SecretNameServer = "kube-apiserver"
	// SecretNameStaticToken is the name of the secret containing static tokens for the kube-apiserver.
	SecretNameStaticToken = "static-token"
	// SecretNameVPNSeed is the name of the secret containing the certificates for the vpn-seed.
	SecretNameVPNSeed = "vpn-seed"
	// SecretNameVPNSeedTLSAuth is the name of the secret containing the TLS auth for the vpn-seed.
	SecretNameVPNSeedTLSAuth = "vpn-seed-tlsauth"

	// ContainerNameKubeAPIServer is the name of the kube-apiserver container.
	ContainerNameKubeAPIServer            = "kube-apiserver"
	containerNameVPNSeed                  = "vpn-seed"
	containerNameAPIServerProxyPodMutator = "apiserver-proxy-pod-mutator"

	volumeNameLibModules      = "modules"
	volumeNameServer          = "kube-apiserver"
	volumeNameVPNSeed         = "vpn-seed"
	volumeNameVPNSeedTLSAuth  = "vpn-seed-tlsauth"
	volumeNameFedora          = "fedora-rhel6-openelec-cabundle"
	volumeNameCentOS          = "centos-rhel7-cabundle"
	volumeNameEtcSSL          = "etc-ssl"
	volumeNameUsrShareCaCerts = "usr-share-cacerts"

	volumeMountPathAdmissionConfiguration = "/etc/kubernetes/admission"
	volumeMountPathHTTPProxy              = "/etc/srv/kubernetes/envoy"
	volumeMountPathLibModules             = "/lib/modules"
	volumeMountPathServer                 = "/srv/kubernetes/apiserver"
	volumeMountPathVPNSeed                = "/srv/secrets/vpn-seed"
	volumeMountPathVPNSeedTLSAuth         = "/srv/secrets/tlsauth"
	volumeMountPathFedora                 = "/etc/pki/tls"
	volumeMountPathCentOS                 = "/etc/pki/ca-trust/extracted/pem"
	volumeMountPathEtcSSL                 = "/etc/ssl"
	volumeMountPathUsrShareCaCerts        = "/usr/share/ca-certificates"
)

func (k *kubeAPIServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	var (
		maxSurge          = intstr.FromString("25%")
		maxUnavailable    = intstr.FromInt(0)
		directoryOrCreate = corev1.HostPathDirectoryOrCreate
	)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), deployment, func() error {
		deployment.Labels = GetLabels()
		if k.values.SNI.Enabled {
			deployment.Labels[v1beta1constants.LabelAPIServerExposure] = v1beta1constants.LabelAPIServerExposureGardenerManaged
		}

		deployment.Spec = appsv1.DeploymentSpec{
			MinReadySeconds:      30,
			RevisionHistoryLimit: pointer.Int32(2),
			Replicas:             k.values.Autoscaling.Replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: getLabels()},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: k.computePodAnnotations(),
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.DeprecatedGardenRole:                v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToShootNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyFromPrometheus:    v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
								Weight: 1,
								PodAffinityTerm: corev1.PodAffinityTerm{
									TopologyKey:   corev1.LabelHostname,
									LabelSelector: &metav1.LabelSelector{MatchLabels: getLabels()},
								},
							}},
						},
					},
					PriorityClassName:             v1beta1constants.PriorityClassNameShootControlPlane,
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: pointer.Int64(30),
					Containers: []corev1.Container{{
						Name:                     ContainerNameKubeAPIServer,
						Image:                    k.values.Images.KubeAPIServer,
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePath:   corev1.TerminationMessagePathDefault,
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						Ports: []corev1.ContainerPort{{
							Name:          "https",
							ContainerPort: Port,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources: k.values.Autoscaling.APIServerResources,
					}},
					Volumes: []corev1.Volume{
						{
							Name: volumeNameServer,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.Server.Name,
								},
							},
						},
					},
				},
			},
		}

		if versionutils.ConstraintK8sGreaterEqual117.Check(k.values.Version) {
			// locations are taken from
			// https://github.com/golang/go/blob/1bb247a469e306c57a5e0eaba788efb8b3b1acef/src/crypto/x509/root_linux.go#L7-L15
			// we cannot be sure on which Node OS the Seed Cluster is running so, it's safer to mount them all
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
				{
					Name:      volumeNameFedora,
					MountPath: volumeMountPathFedora,
					ReadOnly:  true,
				},
				{
					Name:      volumeNameCentOS,
					MountPath: volumeMountPathCentOS,
					ReadOnly:  true,
				},
				{
					Name:      volumeNameEtcSSL,
					MountPath: volumeMountPathEtcSSL,
					ReadOnly:  true,
				},
				{
					Name:      volumeNameUsrShareCaCerts,
					MountPath: volumeMountPathUsrShareCaCerts,
					ReadOnly:  true,
				},
			}...)
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
				{
					Name: volumeNameFedora,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: volumeMountPathFedora,
							Type: &directoryOrCreate,
						},
					},
				},
				{
					Name: volumeNameCentOS,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: volumeMountPathCentOS,
							Type: &directoryOrCreate,
						},
					},
				},
				{
					Name: volumeNameEtcSSL,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: volumeMountPathEtcSSL,
							Type: &directoryOrCreate,
						},
					},
				},
				{
					Name: volumeNameUsrShareCaCerts,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: volumeMountPathUsrShareCaCerts,
							Type: &directoryOrCreate,
						},
					},
				},
			}...)
		}

		if !k.values.VPN.ReversedVPNEnabled {
			deployment.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:  "set-iptable-rules",
				Image: k.values.Images.AlpineIPTables,
				Command: []string{
					"/bin/sh",
					"-c",
					"iptables -A INPUT -i tun0 -p icmp -j ACCEPT && iptables -A INPUT -i tun0 -m state --state NEW -j DROP",
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"NET_ADMIN"},
					},
					Privileged: pointer.Bool(true),
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      volumeNameLibModules,
					MountPath: volumeMountPathLibModules,
				}},
			}}
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeNameLibModules,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/lib/modules"},
				},
			})

			vpnSeedContainer := corev1.Container{
				Name:            containerNameVPNSeed,
				Image:           k.values.Images.VPNSeed,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{
						Name:  "MAIN_VPN_SEED",
						Value: "true",
					},
					{
						Name:  "OPENVPN_PORT",
						Value: "4314",
					},
					{
						Name:  "APISERVER_AUTH_MODE",
						Value: "client-cert",
					},
					{
						Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CA",
						Value: volumeMountPathVPNSeed + "/" + secrets.DataKeyCertificateCA,
					},
					{
						Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CRT",
						Value: volumeMountPathVPNSeed + "/" + secrets.DataKeyCertificate,
					},
					{
						Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_KEY",
						Value: volumeMountPathVPNSeed + "/" + secrets.DataKeyPrivateKey,
					},
					{
						Name:  "SERVICE_NETWORK",
						Value: k.values.VPN.ServiceNetworkCIDR,
					},
					{
						Name:  "POD_NETWORK",
						Value: k.values.VPN.PodNetworkCIDR,
					},
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"NET_ADMIN"},
					},
					Privileged: pointer.Bool(true),
				},
				TerminationMessagePath:   corev1.TerminationMessagePathDefault,
				TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      volumeNameVPNSeed,
						MountPath: volumeMountPathVPNSeed,
					},
					{
						Name:      volumeNameVPNSeedTLSAuth,
						MountPath: volumeMountPathVPNSeedTLSAuth,
					},
				},
			}

			if k.values.VPN.NodeNetworkCIDR != nil {
				vpnSeedContainer.Env = append(vpnSeedContainer.Env, corev1.EnvVar{
					Name:  "NODE_NETWORK",
					Value: *k.values.VPN.NodeNetworkCIDR,
				})
			}

			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, vpnSeedContainer)
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
				{
					Name: volumeNameVPNSeed,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: k.secrets.VPNSeed.Name},
					},
				},
				{
					Name: volumeNameVPNSeedTLSAuth,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: k.secrets.VPNSeedTLSAuth.Name},
					},
				},
			}...)
		}

		if k.values.SNI.PodMutatorEnabled {
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
				Name:  containerNameAPIServerProxyPodMutator,
				Image: k.values.Images.APIServerProxyPodWebhook,
				Args: []string{
					"--apiserver-fqdn=" + k.values.SNI.APIServerFQDN,
					"--host=localhost",
					"--port=9443",
					"--cert-dir=" + volumeMountPathServer,
					"--cert-name=" + secrets.ControlPlaneSecretDataKeyCertificatePEM(SecretNameServer),
					"--key-name=" + secrets.ControlPlaneSecretDataKeyPrivateKey(SecretNameServer),
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128M"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("500M"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      volumeNameServer,
					MountPath: volumeMountPathServer,
				}},
			})
		}

		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	})
	return err
}

func (k *kubeAPIServer) computePodAnnotations() map[string]string {
	out := make(map[string]string)

	for _, s := range k.secrets.all() {
		if s.Secret != nil && s.Name != "" && s.Checksum != "" {
			out["checksum/secret-"+s.Name] = s.Checksum
		}
	}

	return out
}
