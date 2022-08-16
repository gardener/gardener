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

package vpnshoot

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelValue = "vpn-shoot"

	servicePort   int32 = 4314
	containerPort int32 = 1194

	managedResourceName = "shoot-core-vpn-shoot"
	deploymentName      = "vpn-shoot"
	containerName       = "vpn-shoot"
	initContainerName   = "vpn-shoot-init"
	serviceName         = "vpn-shoot"

	secretNameDH = "vpn-shoot-dh"

	volumeName          = "vpn-shoot"
	volumeNameTLSAuth   = "vpn-shoot-tlsauth"
	volumeNameDH        = "vpn-shoot-dh"
	volumeNameDevNetTun = "dev-net-tun"

	volumeMountPathSecret    = "/srv/secrets/vpn-shoot"
	volumeMountPathSecretTLS = "/srv/secrets/tlsauth"
	volumeMountPathSecretDH  = "/srv/secrets/dh"
	volumeMountPathDevNetTun = "/dev/net/tun"
)

// Interface contains functions for a VPNShoot Deployer
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
}

// ReversedVPNValues contains the configuration values for the ReversedVPN.
type ReversedVPNValues struct {
	// Enabled marks whether ReversedVPN is enabled for the shoot
	Enabled bool
	// Header is the header for the ReversedVPN.
	Header string
	// Endpoint is the endpoint for the ReversedVPN.
	Endpoint string
	// OpenVPNPort is the port for the ReversedVPN.
	OpenVPNPort int32
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

// Values is a set of configuration values for the VPNShoot component.
type Values struct {
	// Image is the container image used for vpnShoot.
	Image string
	// PodAnnotations is the set of additional annotations to be used for the pods.
	PodAnnotations map[string]string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// ReversedVPN contains the configuration values for the ReversedVPN.
	ReversedVPN ReversedVPNValues
	// Network contains the configuration values for the network.
	Network NetworkValues
	// PSPDisabled marks whether the PodSecurityPolicy admission plugin is disabled.
	PSPDisabled bool
	// KubernetesVersion is the Kubernetes version of the Shoot.
	KubernetesVersion *semver.Version
}

// New creates a new instance of DeployWaiter for vpnshoot
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &vpnShoot{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type vpnShoot struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
	secrets        Secrets
}

func (v *vpnShoot) Deploy(ctx context.Context) error {
	var (
		config = &secretutils.CertificateSecretConfig{
			Name:                        "vpn-shoot-client",
			CommonName:                  "vpn-shoot-client",
			CertType:                    secretutils.ClientCert,
			SkipPublishingCACertificate: true,
		}
		signingCA = v1beta1constants.SecretNameCAVPN
	)

	if !v.values.ReversedVPN.Enabled {
		config = &secretutils.CertificateSecretConfig{
			Name:                        "vpn-shoot-server",
			CommonName:                  "vpn-shoot-server",
			CertType:                    secretutils.ServerCert,
			SkipPublishingCACertificate: true,
		}
		signingCA = v1beta1constants.SecretNameCAClient
	}

	secretCA, found := v.secretsManager.Get(signingCA)
	if !found {
		return fmt.Errorf("secret %q not found", signingCA)
	}

	secret, err := v.secretsManager.Generate(ctx, config, secretsmanager.SignedByCA(signingCA), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	data, err := v.computeResourcesData(secretCA, secret)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForShoot(ctx, v.client, v.namespace, managedResourceName, false, data); err != nil {
		return err
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObjects(ctx, v.client,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpn-shoot", Namespace: v.namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpn-shoot-client", Namespace: v.namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpn-shoot-server", Namespace: v.namespace}},
	)
}

func (v *vpnShoot) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, v.client, v.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (v *vpnShoot) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *vpnShoot) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *vpnShoot) computeResourcesData(secretCAVPN, secretVPNShoot *corev1.Secret) (map[string][]byte, error) {
	var (
		secretNameSuffix  string
		secretNameTLSAuth string

		secretVPNSeedServerTLSAuth *corev1.Secret
		found                      bool
	)

	if v.values.ReversedVPN.Enabled {
		secretNameSuffix = "client"
		secretNameTLSAuth = vpnseedserver.SecretNameTLSAuth
	} else {
		secretNameSuffix = "server"
		secretNameTLSAuth = kubeapiserver.SecretNameVPNSeedTLSAuth
	}

	secretVPNSeedServerTLSAuth, found = v.secretsManager.Get(secretNameTLSAuth)
	if !found {
		return nil, fmt.Errorf("secret %q not found", secretNameTLSAuth)
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		secretCA = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot-ca",
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretCAVPN.Data,
		}
		secretTLSAuth = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot-tlsauth",
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretVPNSeedServerTLSAuth.Data,
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot-" + secretNameSuffix,
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretVPNShoot.Data,
		}
		secretDH           *corev1.Secret
		service            *corev1.Service
		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	utilruntime.Must(kutil.MakeUnique(secretCA))
	utilruntime.Must(kutil.MakeUnique(secretTLSAuth))
	utilruntime.Must(kutil.MakeUnique(secret))

	if !v.values.ReversedVPN.Enabled {
		secretDH = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretNameDH,
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: v.secrets.DH.Data,
		}
		utilruntime.Must(kutil.MakeUnique(secretDH))

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Selector: getLabels(),
				Type:     corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:       "openvpn",
						Port:       servicePort,
						TargetPort: intstr.FromInt(int(containerPort)),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:gardener.cloud:vpn-seed",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"services"},
					ResourceNames: []string{service.Name},
					Verbs:         []string{"get"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "system:gardener.cloud:vpn-seed",
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: rbacv1.UserKind,
					Name: kubeapiserver.UserNameVPNSeed,
				},
			},
		}
	}

	var (
		intStrMax  = intstr.FromString("100%")
		intStrZero = intstr.FromString("0%")
		vpa        *vpaautoscalingv1.VerticalPodAutoscaler

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-vpn",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: "Allows the VPN to communicate with shoot components and makes the VPN reachable from the seed.",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
				Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
					v1beta1constants.LabelApp:       labelValue,
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				},
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(2),
				Replicas:             pointer.Int32(1),
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       &intStrMax,
						MaxUnavailable: &intStrZero,
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
							v1beta1constants.LabelApp:       labelValue,
							managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
							"type":                          "tunnel",
						},
					},
					Spec: corev1.PodSpec{
						AutomountServiceAccountToken: pointer.Bool(false),
						ServiceAccountName:           serviceAccount.Name,
						PriorityClassName:            "system-cluster-critical",
						DNSPolicy:                    corev1.DNSDefault,
						NodeSelector:                 map[string]string{v1beta1constants.LabelWorkerPoolSystemComponents: "true"},
						Tolerations: []corev1.Toleration{{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
						}},
						InitContainers: v.getInitContainers(),
						Containers: []corev1.Container{
							{
								Name:            containerName,
								Image:           v.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Env:             v.getEnvVars(),
								SecurityContext: &corev1.SecurityContext{
									Privileged: pointer.Bool(!v.values.ReversedVPN.Enabled),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"NET_ADMIN"},
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("100Mi"),
									},
									Limits: v.getResourceLimits(),
								},
								VolumeMounts: v.getVolumeMounts(),
							},
						},
						Volumes: v.getVolumes(secretCA, secret, secretTLSAuth, secretDH),
					},
				},
			},
		}

		podSecurityPolicy *policyv1beta1.PodSecurityPolicy
		clusterRolePSP    *rbacv1.ClusterRole
		roleBindingPSP    *rbacv1.RoleBinding
	)
	utilruntime.Must(references.InjectAnnotations(deployment))

	if !v.values.PSPDisabled {
		podSecurityPolicy = &policyv1beta1.PodSecurityPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.kube-system.vpn-shoot",
				Annotations: map[string]string{
					v1beta1constants.AnnotationSeccompAllowedProfiles: v1beta1constants.AnnotationSeccompAllowedProfilesRuntimeDefaultValue,
					v1beta1constants.AnnotationSeccompDefaultProfile:  v1beta1constants.AnnotationSeccompAllowedProfilesRuntimeDefaultValue,
				},
			},
			Spec: policyv1beta1.PodSecurityPolicySpec{
				Privileged: true,
				Volumes: []policyv1beta1.FSType{
					"secret",
					"emptyDir",
					"projected",
					"hostPath",
				},
				AllowedCapabilities: []corev1.Capability{
					"NET_ADMIN",
				},
				AllowedHostPaths: []policyv1beta1.AllowedHostPath{
					{
						PathPrefix: volumeMountPathDevNetTun,
					},
				},
				RunAsUser: policyv1beta1.RunAsUserStrategyOptions{
					Rule: policyv1beta1.RunAsUserStrategyRunAsAny,
				},
				SELinux: policyv1beta1.SELinuxStrategyOptions{
					Rule: policyv1beta1.SELinuxStrategyRunAsAny,
				},
				SupplementalGroups: policyv1beta1.SupplementalGroupsStrategyOptions{
					Rule: policyv1beta1.SupplementalGroupsStrategyRunAsAny,
				},
				FSGroup: policyv1beta1.FSGroupStrategyOptions{
					Rule: policyv1beta1.FSGroupStrategyRunAsAny,
				},
			},
		}

		clusterRolePSP = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:psp:kube-system:vpn-shoot",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"policy", "extensions"},
					ResourceNames: []string{podSecurityPolicy.Name},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
				},
			},
		}

		roleBindingPSP = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:psp:vpn-shoot",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					resourcesv1alpha1.DeleteOnInvalidUpdate: "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRolePSP.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
	}

	if version.ConstraintK8sGreaterEqual119.Check(v.values.KubernetesVersion) {
		if deployment.Spec.Template.Spec.SecurityContext == nil {
			deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}
		deployment.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	if v.values.VPAEnabled {
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: containerName,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
							ControlledValues: &controlledValues,
						},
					},
				},
			},
		}
	}

	return registry.AddAllAndSerialize(
		secret,
		secretCA,
		secretTLSAuth,
		secretDH,
		serviceAccount,
		networkPolicy,
		deployment,
		clusterRole,
		clusterRoleBinding,
		service,
		vpa,
		podSecurityPolicy,
		clusterRolePSP,
		roleBindingPSP,
	)
}

// Secrets is collection of secrets for the vpn-shoot.
type Secrets struct {
	// DH is a secret containing the Diffie-Hellman credentials.
	DH *component.Secret
}

func (v *vpnShoot) SetSecrets(secrets Secrets) {
	v.secrets = secrets
}

func getLabels() map[string]string {
	return map[string]string{v1beta1constants.LabelApp: labelValue}
}

func (v *vpnShoot) getEnvVars() []corev1.EnvVar {
	envVariables := []corev1.EnvVar{
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
	}
	if v.values.ReversedVPN.Enabled {
		envVariables = append(envVariables,
			corev1.EnvVar{
				Name:  "ENDPOINT",
				Value: v.values.ReversedVPN.Endpoint,
			},
			corev1.EnvVar{
				Name:  "OPENVPN_PORT",
				Value: strconv.Itoa(int(v.values.ReversedVPN.OpenVPNPort)),
			},
			corev1.EnvVar{
				Name:  "REVERSED_VPN_HEADER",
				Value: v.values.ReversedVPN.Header,
			},
			corev1.EnvVar{
				Name:  "DO_NOT_CONFIGURE_KERNEL_SETTINGS",
				Value: "true",
			},
		)
	}
	return envVariables
}

func (v *vpnShoot) getResourceLimits() corev1.ResourceList {
	if v.values.VPAEnabled {
		return corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		}
	}
	return corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("120Mi"),
	}
}

func (v *vpnShoot) getVolumeMounts() []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      volumeName,
			MountPath: volumeMountPathSecret,
		},
		{
			Name:      volumeNameTLSAuth,
			MountPath: volumeMountPathSecretTLS,
		},
	}
	if !v.values.ReversedVPN.Enabled {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameDH,
			MountPath: volumeMountPathSecretDH,
		})
	} else {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameDevNetTun,
			MountPath: volumeMountPathDevNetTun,
		})
	}
	return volumeMounts
}

func (v *vpnShoot) getVolumes(secretCA, secret, secretTLSAuth, secretDH *corev1.Secret) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: pointer.Int32(0400),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretCA.Name,
								},
								Items: []corev1.KeyToPath{{
									Key:  secretutils.DataKeyCertificateBundle,
									Path: "ca.crt",
								}},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secret.Name,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  secretutils.DataKeyCertificate,
										Path: secretutils.DataKeyCertificate,
									},
									{
										Key:  secretutils.DataKeyPrivateKey,
										Path: secretutils.DataKeyPrivateKey,
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
	}
	if !v.values.ReversedVPN.Enabled {
		volumes = append(volumes, corev1.Volume{
			Name: volumeNameDH,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretDH.Name,
					DefaultMode: pointer.Int32(0400),
				},
			},
		})
	} else {
		hostPathCharDev := corev1.HostPathCharDev
		volumes = append(volumes, corev1.Volume{
			Name: volumeNameDevNetTun,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumeMountPathDevNetTun,
					Type: &hostPathCharDev,
				},
			},
		})
	}
	return volumes
}

func (v *vpnShoot) getInitContainers() []corev1.Container {
	if !v.values.ReversedVPN.Enabled {
		return []corev1.Container{}
	}
	return []corev1.Container{
		{
			Name:            initContainerName,
			Image:           v.values.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env: []corev1.EnvVar{
				{
					Name:  "EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS",
					Value: "true",
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged: pointer.Bool(true),
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("30m"),
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				},
			},
		},
	}
}
