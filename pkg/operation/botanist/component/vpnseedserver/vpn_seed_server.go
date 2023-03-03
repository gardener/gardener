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

package vpnseedserver

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Masterminds/semver"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	// GatewayPort is the port exposed by the istio ingress gateway
	GatewayPort = 8132
	// SecretNameTLSAuth is the name of seed server tlsauth Secret.
	SecretNameTLSAuth = "vpn-seed-server-tlsauth"
	// DeploymentName is the name of vpn seed server deployment.
	DeploymentName = v1beta1constants.DeploymentNameVPNSeedServer
	// ServiceName is the name of the vpn seed server service running internally on the control plane in seed.
	ServiceName = DeploymentName
	// EnvoyPort is the port exposed by the envoy proxy on which it receives http proxy/connect requests.
	EnvoyPort = 9443
	// OpenVPNPort is the port exposed by the vpn seed server for tcp tunneling.
	OpenVPNPort = 1194
	// HighAvailabilityReplicaCount is the replica count used when highly available VPN is configured.
	HighAvailabilityReplicaCount = 2
	metricsPortName              = "metrics"
	// MetricsPort is the port metrics can be scraped at.
	MetricsPort = 15000

	secretNameDH            = "vpn-seed-server-dh"
	envoyProxyContainerName = "envoy-proxy"

	fileNameEnvoyConfig = "envoy.yaml"
	fileNameCABundle    = "ca.crt"

	volumeMountPathDevNetTun   = "/dev/net/tun"
	volumeMountPathCerts       = "/srv/secrets/vpn-server"
	volumeMountPathTLSAuth     = "/srv/secrets/tlsauth"
	volumeMountPathDH          = "/srv/secrets/dh"
	volumeMountPathEnvoyConfig = "/etc/envoy"
	volumeMountPathStatusDir   = "/srv/status"

	volumeNameDevNetTun   = "dev-net-tun"
	volumeNameCerts       = "certs"
	volumeNameTLSAuth     = "tlsauth"
	volumeNameDH          = "dh"
	volumeNameEnvoyConfig = "envoy-config"
	volumeNameStatusDir   = "openvpn-status"
)

// Interface contains functions for a vpn-seed-server deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent

	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
	// SetSeedNamespaceObjectUID sets UID for the namespace
	SetSeedNamespaceObjectUID(namespaceUID types.UID)

	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
}

// Secrets is collection of secrets for the vpn-seed-server.
type Secrets struct {
	// DiffieHellmanKey is a secret containing the diffie hellman key.
	DiffieHellmanKey component.Secret
}

// NetworkValues contains the configuration values for the network.
type NetworkValues struct {
	// PodCIDR is the CIDR of the pod network.
	PodCIDR string
	// ServiceCIDR is the CIDR of the service network.
	ServiceCIDR string
	// NodeCIDR is the CIDR of the node network.
	NodeCIDR string
}

// Values is a set of configuration values for the VPNSeedServer component.
type Values struct {
	// ImageAPIServerProxy is the image name of the apiserver-proxy
	ImageAPIServerProxy string
	// ImageVPNSeedServer is the image name of the vpn-seed-server
	ImageVPNSeedServer string
	// KubeAPIServerHost is the FQDN of the kube-apiserver
	KubeAPIServerHost *string
	// Network contains the configuration values for the network.
	Network NetworkValues
	// Replicas is the number of deployment replicas
	Replicas int32
	// HighAvailabilityEnabled marks whether HA is enabled for VPN.
	HighAvailabilityEnabled bool
	// HighAvailabilityNumberOfSeedServers is the number of VPN seed servers used for HA
	HighAvailabilityNumberOfSeedServers int
	// HighAvailabilityNumberOfShootClients is the number of VPN shoot clients used for HA
	HighAvailabilityNumberOfShootClients int
	// SeedVersion is the Kubernetes version of the Seed.
	SeedVersion *semver.Version
}

// New creates a new instance of DeployWaiter for the vpn-seed-server.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	istioNamespaceFunc func() string,
	values Values,
) Interface {
	return &vpnSeedServer{
		client:             client,
		namespace:          namespace,
		secretsManager:     secretsManager,
		values:             values,
		istioNamespaceFunc: istioNamespaceFunc,
	}
}

type vpnSeedServer struct {
	client             client.Client
	namespace          string
	secretsManager     secretsmanager.Interface
	namespaceUID       types.UID
	values             Values
	secrets            Secrets
	istioNamespaceFunc func() string
}

func (v *vpnSeedServer) GetValues() Values {
	return v.values
}

func (v *vpnSeedServer) Deploy(ctx context.Context) error {
	if v.secrets.DiffieHellmanKey.Name == "" || v.secrets.DiffieHellmanKey.Checksum == "" {
		return fmt.Errorf("missing DH secret information")
	}

	var (
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-seed-server-envoy-config",
				Namespace: v.namespace,
			},
			Data: map[string]string{
				fileNameEnvoyConfig: envoyConfig,
			},
		}

		dhSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretNameDH,
				Namespace: v.namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: v.secrets.DiffieHellmanKey.Data,
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	utilruntime.Must(kubernetesutils.MakeUnique(dhSecret))

	secretCAVPN, found := v.secretsManager.Get(v1beta1constants.SecretNameCAVPN)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAVPN)
	}

	secretServer, err := v.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        "vpn-seed-server",
		CommonName:                  "vpn-seed-server",
		DNSNames:                    kubernetesutils.DNSNamesForService(ServiceName, v.namespace),
		CertType:                    secretsutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAVPN), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	secretTLSAuth, err := v.secretsManager.Generate(ctx, &secretsutils.VPNTLSAuthConfig{
		Name: SecretNameTLSAuth,
	}, secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	if err := v.client.Create(ctx, configMap); client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if err := v.client.Create(ctx, dhSecret); client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	// TODO(rfranzke): Delete this in a future release.
	if err := kubernetesutils.DeleteObject(ctx, v.client, v.emptyNetworkPolicy()); err != nil {
		return err
	}

	podTemplate := v.podTemplate(configMap, dhSecret, secretCAVPN, secretServer, secretTLSAuth)
	labels := map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelApp:   DeploymentName,
	}

	if v.values.HighAvailabilityEnabled {
		if err := v.deployStatefulSet(ctx, labels, podTemplate); err != nil {
			return err
		}
		for i := 0; i < int(v.values.Replicas); i++ {
			if err := v.deployService(ctx, &i); err != nil {
				return err
			}
			if err := v.deployDestinationRule(ctx, &i); err != nil {
				return err
			}
		}
		if err := kubernetesutils.DeleteObjects(ctx, v.client,
			v.emptyDeployment(),
			v.emptyService(nil),
			v.emptyDestinationRule(nil),
		); err != nil {
			return err
		}
	} else {
		if err := v.deployDeployment(ctx, labels, podTemplate); err != nil {
			return err
		}
		if err := v.deployService(ctx, nil); err != nil {
			return err
		}
		if err := v.deployDestinationRule(ctx, nil); err != nil {
			return err
		}
		objects := []client.Object{v.emptyStatefulSet()}
		for i := 0; i < v.values.HighAvailabilityNumberOfSeedServers; i++ {
			objects = append(objects, v.emptyService(&i), v.emptyDestinationRule(&i))
		}
		if err := kubernetesutils.DeleteObjects(ctx, v.client, objects...); err != nil {
			return err
		}
	}

	if err := v.deployVPA(ctx); err != nil {
		return err
	}

	return nil
}

func (v *vpnSeedServer) podTemplate(configMap *corev1.ConfigMap, dhSecret, secretCAVPN, secretServer, secretTLSAuth *corev1.Secret) *corev1.PodTemplateSpec {
	hostPathCharDev := corev1.HostPathCharDev
	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				v1beta1constants.GardenRole:                          v1beta1constants.GardenRoleControlPlane,
				v1beta1constants.LabelApp:                            DeploymentName,
				v1beta1constants.LabelNetworkPolicyToShootNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
			},
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: pointer.Bool(false),
			PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane300,
			DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
			Containers: []corev1.Container{
				{
					Name:            DeploymentName,
					Image:           v.values.ImageVPNSeedServer,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{
						{
							Name:          "tcp-tunnel",
							ContainerPort: OpenVPNPort,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_ADMIN",
								"NET_RAW",
							},
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "SERVICE_NETWORK",
							Value: v.values.Network.ServiceCIDR,
						},
						{
							Name:  "POD_NETWORK",
							Value: v.values.Network.PodCIDR,
						},
						{
							Name:  "NODE_NETWORK",
							Value: v.values.Network.NodeCIDR,
						},
						{
							Name: "LOCAL_NODE_IP",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "status.hostIP",
								},
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(OpenVPNPort),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(OpenVPNPort),
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      volumeNameDevNetTun,
							MountPath: volumeMountPathDevNetTun,
						},
						{
							Name:      volumeNameCerts,
							MountPath: volumeMountPathCerts,
						},
						{
							Name:      volumeNameTLSAuth,
							MountPath: volumeMountPathTLSAuth,
						},
						{
							Name:      volumeNameDH,
							MountPath: volumeMountPathDH,
						},
					},
				},
			},
			TerminationGracePeriodSeconds: pointer.Int64(30),
			Volumes: []corev1.Volume{
				{
					Name: volumeNameDevNetTun,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: volumeMountPathDevNetTun,
							Type: &hostPathCharDev,
						},
					},
				},
				{
					Name: volumeNameCerts,
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: pointer.Int32(420),
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretCAVPN.Name,
										},
										Items: []corev1.KeyToPath{{
											Key:  secretsutils.DataKeyCertificateBundle,
											Path: fileNameCABundle,
										}},
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretServer.Name,
										},
										Items: []corev1.KeyToPath{
											{
												Key:  secretsutils.DataKeyCertificate,
												Path: secretsutils.DataKeyCertificate,
											},
											{
												Key:  secretsutils.DataKeyPrivateKey,
												Path: secretsutils.DataKeyPrivateKey,
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Name: volumeNameTLSAuth,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  secretTLSAuth.Name,
							DefaultMode: pointer.Int32(0400),
						},
					},
				},
				{
					Name: volumeNameDH,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  dhSecret.Name,
							DefaultMode: pointer.Int32(0400),
						},
					},
				},
			},
		},
	}

	if !v.values.HighAvailabilityEnabled {
		template.Spec.Containers = append(template.Spec.Containers, corev1.Container{
			Name:            envoyProxyContainerName,
			Image:           v.values.ImageAPIServerProxy,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"all",
					},
				},
			},
			Command: []string{
				"envoy",
				"--concurrency",
				"2",
				"-c",
				fmt.Sprintf("%s/%s", volumeMountPathEnvoyConfig, fileNameEnvoyConfig),
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(EnvoyPort),
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(EnvoyPort),
					},
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20m"),
					corev1.ResourceMemory: resource.MustParse("20Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("850M"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeNameCerts,
					MountPath: volumeMountPathCerts,
				},
				{
					Name:      volumeNameEnvoyConfig,
					MountPath: volumeMountPathEnvoyConfig,
				},
			},
		})
		template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
			Name: volumeNameEnvoyConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMap.Name,
					},
				},
			},
		})
	} else {
		statusPath := filepath.Join(volumeMountPathStatusDir, "openvpn.status")
		template.Spec.Containers = append(template.Spec.Containers, corev1.Container{
			Name:            "openvpn-exporter",
			Image:           v.values.ImageVPNSeedServer,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"/openvpn-exporter",
				"-openvpn.status_paths",
				statusPath,
				"-web.listen-address",
				fmt.Sprintf(":%d", MetricsPort),
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          metricsPortName,
					ContainerPort: MetricsPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"all",
					},
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(MetricsPort),
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(MetricsPort),
					},
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeNameStatusDir,
					MountPath: volumeMountPathStatusDir,
				},
			},
		})
		template.Spec.Containers[0].Env = append(template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "OPENVPN_STATUS_PATH",
			Value: statusPath,
		})
		template.Spec.Containers[0].VolumeMounts = append(template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameStatusDir,
			MountPath: volumeMountPathStatusDir,
		})
		template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
			Name: volumeNameStatusDir,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	if v.values.HighAvailabilityEnabled {
		template.Spec.Containers[0].Env = append(
			template.Spec.Containers[0].Env,
			[]corev1.EnvVar{
				{
					Name:  "CLIENT_TO_CLIENT",
					Value: "true",
				},
				{
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  "HA_VPN_CLIENTS",
					Value: fmt.Sprintf("%d", v.values.HighAvailabilityNumberOfShootClients),
				},
			}...)
	}

	return template
}

func (v *vpnSeedServer) deployStatefulSet(ctx context.Context, labels map[string]string, template *corev1.PodTemplateSpec) error {
	sts := v.emptyStatefulSet()
	podLabels := map[string]string{
		v1beta1constants.LabelApp: DeploymentName,
	}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, sts, func() error {
		sts.Labels = labels
		sts.Spec = appsv1.StatefulSetSpec{
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			Replicas:             pointer.Int32(v.values.Replicas),
			RevisionHistoryLimit: pointer.Int32(1),
			Selector:             &metav1.LabelSelector{MatchLabels: podLabels},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: *template,
		}
		utilruntime.Must(references.InjectAnnotations(sts))
		return nil
	})
	if err != nil {
		return err
	}
	return v.deployPodDisruptionBudget(ctx, podLabels)
}

func (v *vpnSeedServer) deployPodDisruptionBudget(ctx context.Context, podLabels map[string]string) error {
	var (
		pdbMaxUnavailable = intstr.FromInt(1)
		pdbSelector       = &metav1.LabelSelector{MatchLabels: podLabels}
	)

	obj := v.emptyPodDisruptionBudget()

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, obj, func() error {
		switch pdb := obj.(type) {
		case *policyv1.PodDisruptionBudget:
			pdb.Labels = podLabels
			pdb.Spec = policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &pdbMaxUnavailable,
				Selector:       pdbSelector,
			}
		case *policyv1beta1.PodDisruptionBudget:
			pdb.Labels = podLabels
			pdb.Spec = policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &pdbMaxUnavailable,
				Selector:       pdbSelector,
			}
		}
		return nil
	})

	return err
}

func (v *vpnSeedServer) deployDeployment(ctx context.Context, labels map[string]string, template *corev1.PodTemplateSpec) error {
	deployment := v.emptyDeployment()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, deployment, func() error {
		maxSurge := intstr.FromInt(100)
		maxUnavailable := intstr.FromInt(0)
		deployment.Labels = labels
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas:             pointer.Int32(v.values.Replicas),
			RevisionHistoryLimit: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				v1beta1constants.LabelApp: DeploymentName,
			}},
			Strategy: appsv1.DeploymentStrategy{
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: *template,
		}
		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	})
	return err
}

func (v *vpnSeedServer) deployService(ctx context.Context, idx *int) error {
	service := v.emptyService(idx)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, service, func() error {
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.istio.io/exportTo", "*")

		metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(service,
			metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
			metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: v1beta1constants.LabelExposureClassHandlerName, Operator: metav1.LabelSelectorOpExists}}}))
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkingv1.NetworkPolicyPort{Port: utils.IntStrPtrFromInt(MetricsPort), Protocol: utils.ProtocolPtr(corev1.ProtocolTCP)}))

		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       DeploymentName,
				Port:       OpenVPNPort,
				TargetPort: intstr.FromInt(OpenVPNPort),
			},
			{
				Name:       "http-proxy",
				Port:       EnvoyPort,
				TargetPort: intstr.FromInt(EnvoyPort),
			},
			{
				Name:       metricsPortName,
				Port:       MetricsPort,
				TargetPort: intstr.FromInt(MetricsPort),
			},
		}

		if idx == nil {
			service.Spec.Selector = map[string]string{
				v1beta1constants.LabelApp: DeploymentName,
			}
		} else {
			service.Spec.Selector = map[string]string{
				"statefulset.kubernetes.io/pod-name": v.indexedName(idx),
			}
		}

		return nil
	})
	return err
}

func (v *vpnSeedServer) deployDestinationRule(ctx context.Context, idx *int) error {
	destinationRule := v.emptyDestinationRule(idx)
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, destinationRule, func() error {
		destinationRule.Spec = istionetworkingv1beta1.DestinationRule{
			ExportTo: []string{"*"},
			Host:     fmt.Sprintf("%s.%s.svc.cluster.local", v.indexedName(idx), v.namespace),
			TrafficPolicy: &istionetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istionetworkingv1beta1.ConnectionPoolSettings{
					Tcp: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
						MaxConnections: 5000,
						TcpKeepalive: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
							Interval: &durationpb.Duration{
								Seconds: 75,
							},
							Time: &durationpb.Duration{
								Seconds: 7200,
							},
						},
					},
				},
				LoadBalancer: &istionetworkingv1beta1.LoadBalancerSettings{
					LocalityLbSetting: &istionetworkingv1beta1.LocalityLoadBalancerSetting{
						Enabled:          &wrapperspb.BoolValue{Value: true},
						FailoverPriority: []string{corev1.LabelTopologyZone},
					},
				},
				// OutlierDetection is required for locality settings to take effect
				OutlierDetection: &istionetworkingv1beta1.OutlierDetection{
					MinHealthPercent: 0,
				},
				Tls: &istionetworkingv1beta1.ClientTLSSettings{
					Mode: istionetworkingv1beta1.ClientTLSSettings_DISABLE,
				},
			},
		}
		return nil
	})
	return err
}

func (v *vpnSeedServer) deployVPA(ctx context.Context) error {
	var (
		vpa              = v.emptyVPA()
		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	)
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       DeploymentName,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName: DeploymentName,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
					ControlledValues: &controlledValues,
				},
				{
					ContainerName: envoyProxyContainerName,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("20Mi"),
					},
					ControlledValues: &controlledValues,
				},
			},
		}
		return nil
	})
	return err
}

func (v *vpnSeedServer) Destroy(ctx context.Context) error {
	objects := []client.Object{
		v.emptyNetworkPolicy(),
		v.emptyDeployment(),
		v.emptyStatefulSet(),
		v.emptyDestinationRule(nil),
		v.emptyService(nil),
		v.emptyVPA(),
		v.emptyEnvoyFilter(),
		v.emptyPodDisruptionBudget(),
	}
	for i := 0; i < v.values.HighAvailabilityNumberOfSeedServers; i++ {
		objects = append(objects, v.emptyDestinationRule(&i), v.emptyService(&i))
	}
	return kubernetesutils.DeleteObjects(ctx, v.client, objects...)
}

func (v *vpnSeedServer) Wait(_ context.Context) error        { return nil }
func (v *vpnSeedServer) WaitCleanup(_ context.Context) error { return nil }

func (v *vpnSeedServer) SetSecrets(secrets Secrets) { v.secrets = secrets }

func (v *vpnSeedServer) SetSeedNamespaceObjectUID(namespaceUID types.UID) {
	v.namespaceUID = namespaceUID
}

func (v *vpnSeedServer) indexedName(idx *int) string {
	if idx == nil {
		return DeploymentName
	}
	return fmt.Sprintf("%s-%d", DeploymentName, *idx)
}

func (v *vpnSeedServer) emptyService(idx *int) *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: v.indexedName(idx), Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyPodDisruptionBudget() client.Object {
	pdbObjectMeta := metav1.ObjectMeta{
		Name:      DeploymentName,
		Namespace: v.namespace,
	}

	if version.ConstraintK8sGreaterEqual121.Check(v.values.SeedVersion) {
		return &policyv1.PodDisruptionBudget{
			ObjectMeta: pdbObjectMeta,
		}
	}
	return &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: pdbObjectMeta,
	}
}

func (v *vpnSeedServer) emptyNetworkPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-vpn-seed-server", Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyDestinationRule(idx *int) *networkingv1beta1.DestinationRule {
	return &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: v.indexedName(idx), Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName + "-vpa", Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyEnvoyFilter() *networkingv1alpha3.EnvoyFilter {
	return &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Name: v.namespace + "-vpn", Namespace: v.istioNamespaceFunc()}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelApp:   DeploymentName,
	}
}

var envoyConfig = `static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        protocol: TCP
        address: 0.0.0.0
        port_value: ` + fmt.Sprintf("%d", EnvoyPort) + `
    listener_filters:
    - name: "envoy.filters.listener.tls_inspector"
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.filters.listener.tls_inspector.v3.TlsInspector
    filter_chains:
    - transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
          common_tls_context:
            tls_certificates:
            - certificate_chain: { filename: "` + volumeMountPathCerts + `/` + secretsutils.DataKeyCertificate + `" }
              private_key: { filename: "` + volumeMountPathCerts + `/` + secretsutils.DataKeyPrivateKey + `" }
            validation_context:
              trusted_ca:
                filename: ` + volumeMountPathCerts + `/` + fileNameCABundle + `
      filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          access_log:
          - name: envoy.access_loggers.stdout
            filter:
              or_filter:
                filters:
                - status_code_filter:
                    comparison:
                      op: GE
                      value:
                        default_value: 500
                        runtime_key: "null"
                - duration_filter:
                    comparison:
                      op: GE
                      value:
                        default_value: 1000
                        runtime_key: "null"
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
              log_format:
                text_format_source:
                  inline_string: "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% rx %BYTES_SENT% tx %DURATION%ms \"%DOWNSTREAM_REMOTE_ADDRESS%\" \"%REQ(X-REQUEST-ID)%\" \"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\"\n"
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains:
              - "*"
              routes:
              - match:
                  connect_matcher: {}
                route:
                  cluster: dynamic_forward_proxy_cluster
                  upgrade_configs:
                  - upgrade_type: CONNECT
                    connect_config: {}
          http_filters:
          - name: envoy.filters.http.dynamic_forward_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.dynamic_forward_proxy.v3.FilterConfig
              dns_cache_config:
                name: dynamic_forward_proxy_cache_config
                dns_lookup_family: V4_ONLY
                max_hosts: 8192
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
          http_protocol_options:
            accept_http_10: true
          upgrade_configs:
          - upgrade_type: CONNECT
  - name: metrics_listener
    address:
      socket_address:
        address: 0.0.0.0
        port_value: ` + fmt.Sprintf("%d", MetricsPort) + `
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: stats_server
          route_config:
            virtual_hosts:
            - name: admin_interface
              domains:
              - "*"
              routes:
              - match:
                  prefix: "/metrics"
                  headers:
                  - name: ":method"
                    string_match:
                      exact: GET
                route:
                  cluster: prometheus_stats
                  prefix_rewrite: "/stats/prometheus"
          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
  clusters:
  - name: dynamic_forward_proxy_cluster
    connect_timeout: 20s
    circuitBreakers:
      thresholds:
      - maxConnections: 8192
    lb_policy: CLUSTER_PROVIDED
    cluster_type:
      name: envoy.clusters.dynamic_forward_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.clusters.dynamic_forward_proxy.v3.ClusterConfig
        dns_cache_config:
          name: dynamic_forward_proxy_cache_config
          dns_lookup_family: V4_ONLY
          max_hosts: 8192
  - name: prometheus_stats
    connect_timeout: 0.25s
    type: static
    load_assignment:
      cluster_name: prometheus_stats
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              pipe:
                path: /home/nonroot/envoy.admin
admin:
  address:
    pipe:
      path: /home/nonroot/envoy.admin`
