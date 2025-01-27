// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedserver

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// GatewayPort is the port exposed by the istio ingress gateway
	GatewayPort = 8132
	// SecretNameTLSAuth is the name of seed server tlsauth Secret.
	SecretNameTLSAuth = "vpn-seed-server-tlsauth" // #nosec G101 -- No credential.
	deploymentName    = v1beta1constants.DeploymentNameVPNSeedServer
	// ServiceName is the name of the vpn seed server service running internally on the control plane in seed.
	ServiceName = deploymentName
	// EnvoyPort is the port exposed by the envoy proxy on which it receives http proxy/connect requests.
	EnvoyPort = 9443
	// OpenVPNPort is the port exposed by the vpn seed server for tcp tunneling.
	OpenVPNPort = 1194
	// HighAvailabilityReplicaCount is the replica count used when highly available VPN is configured.
	HighAvailabilityReplicaCount = 2
	metricsPortName              = "metrics"
	metricsPort                  = 15000

	envoyProxyContainerName = "envoy-proxy"

	fileNameEnvoyConfig = "envoy.yaml"
	fileNameCABundle    = "ca.crt"

	volumeMountPathDevNetTun   = "/dev/net/tun"
	volumeMountPathCerts       = "/srv/secrets/vpn-server"
	volumeMountPathTLSAuth     = "/srv/secrets/tlsauth"
	volumeMountPathEnvoyConfig = "/etc/envoy"
	volumeMountPathStatusDir   = "/srv/status"

	volumeNameDevNetTun   = "dev-net-tun"
	volumeNameCerts       = "certs"
	volumeNameTLSAuth     = "tlsauth"
	volumeNameEnvoyConfig = "envoy-config"
	volumeNameStatusDir   = "openvpn-status"
)

// Interface contains functions for a vpn-seed-server deployer.
type Interface interface {
	component.DeployWaiter

	SetNodeNetworkCIDRs(nodes []net.IPNet)
	SetServiceNetworkCIDRs(services []net.IPNet)
	SetPodNetworkCIDRs(pods []net.IPNet)
	// SetSeedNamespaceObjectUID sets UID for the namespace
	SetSeedNamespaceObjectUID(namespaceUID types.UID)

	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
}

// NetworkValues contains the configuration values for the network.
type NetworkValues struct {
	// PodCIDRs are the CIDRs of the pod network.
	PodCIDRs []net.IPNet
	// ServiceCIDR are the CIDRs of the service network.
	ServiceCIDRs []net.IPNet
	// NodeCIDRs are the CIDRs of the node network.
	NodeCIDRs []net.IPNet
	// IPFamilies are the IPFamilies of the shoot
	IPFamilies []gardencorev1beta1.IPFamily
}

// Values is a set of configuration values for the VPNSeedServer component.
type Values struct {
	// RuntimeKubernetesVersion is the Kubernetes version of the runtime cluster.
	RuntimeKubernetesVersion *semver.Version
	// ImageAPIServerProxy is the image name of the apiserver-proxy.
	ImageAPIServerProxy string
	// ImageVPNSeedServer is the image name of the vpn-seed-server.
	ImageVPNSeedServer string
	// KubeAPIServerHost is the FQDN of the kube-apiserver.
	KubeAPIServerHost *string
	// Network contains the configuration values for the network.
	Network NetworkValues
	// Replicas is the number of deployment replicas.
	Replicas int32
	// HighAvailabilityEnabled marks whether HA is enabled for VPN.
	HighAvailabilityEnabled bool
	// HighAvailabilityNumberOfSeedServers is the number of VPN seed servers used for HA.
	HighAvailabilityNumberOfSeedServers int
	// HighAvailabilityNumberOfShootClients is the number of VPN shoot clients used for HA.
	HighAvailabilityNumberOfShootClients int
	// VPAUpdateDisabled indicates whether the vertical pod autoscaler update should be disabled.
	VPAUpdateDisabled bool
	// DisableNewVPN disable new VPN implementation.
	// TODO(MartinWeindel) Remove after feature gate `NewVPN` gets promoted to GA.
	DisableNewVPN bool
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
	istioNamespaceFunc func() string
}

func (v *vpnSeedServer) GetValues() Values {
	return v.values
}

func (v *vpnSeedServer) Deploy(ctx context.Context) error {
	var (
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-seed-server-envoy-config",
				Namespace: v.namespace,
			},
			Data: map[string]string{
				fileNameEnvoyConfig: v.getEnvoyConfig(),
			},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

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

	podTemplate := v.podTemplate(configMap, secretCAVPN, secretServer, secretTLSAuth)
	labels := getLabels()

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

	if err := v.deployScrapeConfig(ctx); err != nil {
		return err
	}

	return v.deployVPA(ctx)
}

func (v *vpnSeedServer) podTemplate(configMap *corev1.ConfigMap, secretCAVPN, secretServer, secretTLSAuth *corev1.Secret) *corev1.PodTemplateSpec {
	hostPathCharDev := corev1.HostPathCharDev
	var (
		ipFamilies   []string
		serviceCIDRs []string
		podCIDRs     []string
		nodeCIDRs    []string
	)

	for _, v := range v.values.Network.IPFamilies {
		ipFamilies = append(ipFamilies, string(v))
	}

	nodeNetwork := ""
	if len(v.values.Network.NodeCIDRs) > 0 {
		nodeNetwork = v.values.Network.NodeCIDRs[0].String()
	}

	for _, v := range v.values.Network.ServiceCIDRs {
		serviceCIDRs = append(serviceCIDRs, v.String())
	}

	for _, v := range v.values.Network.PodCIDRs {
		podCIDRs = append(podCIDRs, v.String())
	}

	for _, v := range v.values.Network.NodeCIDRs {
		nodeCIDRs = append(nodeCIDRs, v.String())
	}

	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: utils.MergeStringMaps(getLabels(), map[string]string{
				v1beta1constants.LabelNetworkPolicyToDNS:                                                                    v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPrivateNetworks:                                                        v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
			}),
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane300,
			DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
			Containers: []corev1.Container{
				{
					Name:            deploymentName,
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
						AllowPrivilegeEscalation: ptr.To(false),
					},
					Env: []corev1.EnvVar{
						{
							Name:  "IP_FAMILIES",
							Value: strings.Join(ipFamilies, ","),
						},
						{
							Name:  "SERVICE_NETWORK",
							Value: v.values.Network.ServiceCIDRs[0].String(),
						},
						{
							Name:  "POD_NETWORK",
							Value: v.values.Network.PodCIDRs[0].String(),
						},
						{
							Name:  "NODE_NETWORK",
							Value: nodeNetwork,
						},
						{
							Name:  "SERVICE_NETWORKS",
							Value: strings.Join(serviceCIDRs, ","),
						},
						{
							Name:  "POD_NETWORKS",
							Value: strings.Join(podCIDRs, ","),
						},
						{
							Name:  "NODE_NETWORKS",
							Value: strings.Join(nodeCIDRs, ","),
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
								Port: intstr.FromInt32(OpenVPNPort),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt32(OpenVPNPort),
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("20Mi"),
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
					},
				},
			},
			TerminationGracePeriodSeconds: ptr.To[int64](30),
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
							DefaultMode: ptr.To[int32](420),
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
							DefaultMode: ptr.To[int32](0400),
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
				AllowPrivilegeEscalation: ptr.To(false),
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
						Port: intstr.FromInt32(EnvoyPort),
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt32(EnvoyPort),
					},
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
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
		template.Spec.Containers[0].Env = append(
			template.Spec.Containers[0].Env,
			[]corev1.EnvVar{
				{
					Name:  "OPENVPN_STATUS_PATH",
					Value: filepath.Join(volumeMountPathStatusDir, "openvpn.status"),
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
					Name:  "IS_HA",
					Value: "true",
				},
				{
					Name:  "HA_VPN_CLIENTS",
					Value: strconv.Itoa(v.values.HighAvailabilityNumberOfShootClients),
				},
			}...)
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

		exporterContainer := corev1.Container{
			Name:            "openvpn-exporter",
			Image:           v.values.ImageVPNSeedServer,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"/bin/vpn-server",
				"exporter",
			},
			Env: []corev1.EnvVar{
				{
					Name:  "OPENVPN_STATUS_PATH",
					Value: filepath.Join(volumeMountPathStatusDir, "openvpn.status"),
				},
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          metricsPortName,
					ContainerPort: metricsPort,
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
						Port: intstr.FromInt32(metricsPort),
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt32(metricsPort),
					},
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("10Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeNameStatusDir,
					MountPath: volumeMountPathStatusDir,
				},
			},
		}

		if v.values.DisableNewVPN {
			statusPath := filepath.Join(volumeMountPathStatusDir, "openvpn.status")
			exporterContainer.Command = []string{
				"/openvpn-exporter",
				"-openvpn.status_paths",
				statusPath,
				"-web.listen-address",
				fmt.Sprintf(":%d", metricsPort),
			}
			exporterContainer.Env = nil
		}

		template.Spec.Containers = append(template.Spec.Containers, exporterContainer)
	}

	if !v.values.DisableNewVPN {
		template.Spec.InitContainers = []corev1.Container{
			{
				Name:            "setup",
				Image:           v.values.ImageVPNSeedServer,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"/bin/vpn-server",
					"setup",
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged: ptr.To(true),
				},
			},
		}
	}

	return template
}

func (v *vpnSeedServer) deployStatefulSet(ctx context.Context, labels map[string]string, template *corev1.PodTemplateSpec) error {
	sts := v.emptyStatefulSet()
	podLabels := map[string]string{
		v1beta1constants.LabelApp: deploymentName,
	}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, sts, func() error {
		sts.Labels = labels
		sts.Spec = appsv1.StatefulSetSpec{
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			Replicas:             ptr.To(v.values.Replicas),
			RevisionHistoryLimit: ptr.To[int32](1),
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
	pdb := v.emptyPodDisruptionBudget()

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, pdb, func() error {
		pdb.Labels = podLabels
		pdb.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt32(1)),
			Selector:       &metav1.LabelSelector{MatchLabels: podLabels},
		}

		kubernetesutils.SetAlwaysAllowEviction(pdb, v.values.RuntimeKubernetesVersion)

		return nil
	})

	return err
}

func (v *vpnSeedServer) deployDeployment(ctx context.Context, labels map[string]string, template *corev1.PodTemplateSpec) error {
	deployment := v.emptyDeployment()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, deployment, func() error {
		maxSurge := intstr.FromInt32(100)
		maxUnavailable := intstr.FromInt32(0)
		deployment.Labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook: "true",
		})
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas:             ptr.To(v.values.Replicas),
			RevisionHistoryLimit: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				v1beta1constants.LabelApp: deploymentName,
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
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromInt32(metricsPort)), Protocol: ptr.To(corev1.ProtocolTCP)}))

		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       deploymentName,
				Port:       OpenVPNPort,
				TargetPort: intstr.FromInt32(OpenVPNPort),
			},
			{
				Name:       "http-proxy",
				Port:       EnvoyPort,
				TargetPort: intstr.FromInt32(EnvoyPort),
			},
			{
				Name:       metricsPortName,
				Port:       metricsPort,
				TargetPort: intstr.FromInt32(metricsPort),
			},
		}

		if idx == nil {
			service.Spec.Selector = map[string]string{
				v1beta1constants.LabelApp: deploymentName,
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

func (v *vpnSeedServer) deployScrapeConfig(ctx context.Context) error {
	var (
		jobName, serviceNameRegexSuffix = "reversed-vpn-envoy-side-car", ""
		allowedMetrics                  = []string{
			"envoy_cluster_external_upstream_rq",
			"envoy_cluster_external_upstream_rq_completed",
			"envoy_cluster_external_upstream_rq_xx",
			"envoy_cluster_lb_healthy_panic",
			"envoy_cluster_original_dst_host_invalid",
			"envoy_cluster_upstream_cx_active",
			"envoy_cluster_upstream_cx_connect_attempts_exceeded",
			"envoy_cluster_upstream_cx_connect_fail",
			"envoy_cluster_upstream_cx_connect_timeout",
			"envoy_cluster_upstream_cx_max_requests",
			"envoy_cluster_upstream_cx_none_healthy",
			"envoy_cluster_upstream_cx_overflow",
			"envoy_cluster_upstream_cx_pool_overflow",
			"envoy_cluster_upstream_cx_protocol_error",
			"envoy_cluster_upstream_cx_rx_bytes_total",
			"envoy_cluster_upstream_cx_total",
			"envoy_cluster_upstream_cx_tx_bytes_total",
			"envoy_cluster_upstream_rq",
			"envoy_cluster_upstream_rq_completed",
			"envoy_cluster_upstream_rq_max_duration_reached",
			"envoy_cluster_upstream_rq_pending_overflow",
			"envoy_cluster_upstream_rq_per_try_timeout",
			"envoy_cluster_upstream_rq_retry",
			"envoy_cluster_upstream_rq_retry_limit_exceeded",
			"envoy_cluster_upstream_rq_retry_overflow",
			"envoy_cluster_upstream_rq_rx_reset",
			"envoy_cluster_upstream_rq_timeout",
			"envoy_cluster_upstream_rq_total",
			"envoy_cluster_upstream_rq_tx_reset",
			"envoy_cluster_upstream_rq_xx",
			"envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_attempt",
			"envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_failure",
			"envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_success",
			"envoy_dns_cache_dynamic_forward_proxy_cache_config_host_overflow",
			"envoy_dns_cache_dynamic_forward_proxy_cache_config_num_hosts",
			"envoy_http_downstream_cx_rx_bytes_total",
			"envoy_http_downstream_cx_total",
			"envoy_http_downstream_cx_tx_bytes_total",
			"envoy_http_downstream_rq_xx",
			"envoy_http_no_route",
			"envoy_http_rq_total",
			"envoy_listener_http_downstream_rq_xx",
			"envoy_server_memory_allocated",
			"envoy_server_memory_heap_size",
			"envoy_server_memory_physical_size",
			"envoy_cluster_upstream_cx_connect_ms_bucket",
			"envoy_cluster_upstream_cx_connect_ms_sum",
			"envoy_cluster_upstream_cx_length_ms_bucket",
			"envoy_cluster_upstream_cx_length_ms_sum",
			"envoy_http_downstream_cx_length_ms_bucket",
			"envoy_http_downstream_cx_length_ms_sum",
		}
	)

	if v.values.HighAvailabilityEnabled {
		jobName, serviceNameRegexSuffix = "openvpn-server-exporter", "-[0-2]"
		allowedMetrics = []string{
			"openvpn_server_client_received_bytes_total",
			"openvpn_server_client_sent_bytes_total",
			"openvpn_server_route_last_reference_time_seconds",
			"openvpn_status_update_time_seconds",
			"openvpn_up",
		}
	}

	scrapeConfig := v.emptyScrapeConfig()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = monitoringv1alpha1.ScrapeConfigSpec{
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:       monitoringv1alpha1.KubernetesRoleService,
				Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{v.namespace}},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					Action:      "replace",
					Replacement: ptr.To(jobName),
					TargetLabel: "job",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_service_port_name"},
					Action:       "keep",
					Regex:        ServiceName + serviceNameRegexSuffix + `;` + metricsPortName,
				},
			},
			MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(allowedMetrics...),
		}

		if v.values.HighAvailabilityEnabled {
			scrapeConfig.Spec.MetricRelabelConfigs = append(scrapeConfig.Spec.MetricRelabelConfigs,
				monitoringv1.RelabelConfig{
					SourceLabels: []monitoringv1.LabelName{"instance"},
					Action:       "replace",
					Regex:        `([^.]+).+`,
					TargetLabel:  "service",
				},
				monitoringv1.RelabelConfig{
					SourceLabels: []monitoringv1.LabelName{"real_address"},
					Action:       "replace",
					Regex:        `([^:]+).+`,
					TargetLabel:  "real_ip",
				},
				monitoringv1.RelabelConfig{
					Regex:  "username",
					Action: "labeldrop",
				},
			)
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

	targetRefKind := "Deployment"
	if v.values.HighAvailabilityEnabled {
		targetRefKind = "StatefulSet"
	}

	if v.values.VPAUpdateDisabled {
		vpaUpdateMode = vpaautoscalingv1.UpdateModeOff
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       targetRefKind,
			Name:       deploymentName,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName: deploymentName,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("20Mi"),
					},
					ControlledValues: &controlledValues,
				},
				{
					ContainerName: envoyProxyContainerName,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("100Mi"),
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
		v.emptyScrapeConfig(),
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

func (v *vpnSeedServer) SetSeedNamespaceObjectUID(namespaceUID types.UID) {
	v.namespaceUID = namespaceUID
}

func (v *vpnSeedServer) SetNodeNetworkCIDRs(nodes []net.IPNet) {
	v.values.Network.NodeCIDRs = nodes
}

func (v *vpnSeedServer) SetServiceNetworkCIDRs(services []net.IPNet) {
	v.values.Network.ServiceCIDRs = services
}

func (v *vpnSeedServer) SetPodNetworkCIDRs(pods []net.IPNet) {
	v.values.Network.PodCIDRs = pods
}

func (v *vpnSeedServer) indexedName(idx *int) string {
	if idx == nil {
		return deploymentName
	}
	return fmt.Sprintf("%s-%d", deploymentName, *idx)
}

func (v *vpnSeedServer) emptyService(idx *int) *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: v.indexedName(idx), Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta(deploymentName, v.namespace, shoot.Label)}
}

func (v *vpnSeedServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyDestinationRule(idx *int) *networkingv1beta1.DestinationRule {
	return &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: v.indexedName(idx), Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: deploymentName + "-vpa", Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyEnvoyFilter() *networkingv1alpha3.EnvoyFilter {
	return &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Name: v.namespace + "-vpn", Namespace: v.istioNamespaceFunc()}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelApp:   deploymentName,
	}
}

func (v *vpnSeedServer) getEnvoyConfig() string {
	var (
		listenAddress   = "0.0.0.0"
		listenAddressV6 = "::"
		dnsLookupFamily = "ALL"
	)

	return `static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        protocol: TCP
        address: "` + listenAddress + `"
        port_value: ` + strconv.Itoa(EnvoyPort) + `
    additional_addresses:
    - address:
        socket_address:
          address: "` + listenAddressV6 + `"
          port_value: ` + strconv.Itoa(EnvoyPort) + `
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
                dns_lookup_family: ` + dnsLookupFamily + `
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
        address: "` + listenAddress + `"
        port_value: ` + strconv.Itoa(metricsPort) + `
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
          dns_lookup_family: ` + dnsLookupFamily + `
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
}
