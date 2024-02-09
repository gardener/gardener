// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// LabelValue is used as value for LabelApp.
	LabelValue = "vpn-shoot"

	managedResourceName = "shoot-core-vpn-shoot"
	deploymentName      = "vpn-shoot"
	containerName       = "vpn-shoot"
	initContainerName   = "vpn-shoot-init"
	serviceName         = "vpn-shoot"

	volumeName          = "vpn-shoot"
	volumeNameTLSAuth   = "vpn-shoot-tlsauth"
	volumeNameDevNetTun = "dev-net-tun"

	volumeMountPathSecret    = "/srv/secrets/vpn-client"
	volumeMountPathSecretTLS = "/srv/secrets/tlsauth"
	volumeMountPathDevNetTun = "/dev/net/tun"
)

// Interface contains functions for a VPNShoot Deployer
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
}

// ReversedVPNValues contains the configuration values for the ReversedVPN.
type ReversedVPNValues struct {
	// Header is the header for the ReversedVPN.
	Header string
	// Endpoint is the endpoint for the ReversedVPN.
	Endpoint string
	// OpenVPNPort is the port for the ReversedVPN.
	OpenVPNPort int32
	// IPFamilies are the IPFamilies of the shoot.
	IPFamilies []gardencorev1beta1.IPFamily
}

// Values is a set of configuration values for the VPNShoot component.
type Values struct {
	// Image is the container image used for vpnShoot.
	Image string
	// KubernetesVersion is the Kubernetes version of the Shoot.
	KubernetesVersion *semver.Version
	// PodAnnotations is the set of additional annotations to be used for the pods.
	PodAnnotations map[string]string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// ReversedVPN contains the configuration values for the ReversedVPN.
	ReversedVPN ReversedVPNValues
	// HighAvailabilityEnabled marks whether HA is enabled for VPN.
	HighAvailabilityEnabled bool
	// HighAvailabilityNumberOfSeedServers is the number of VPN seed servers used for HA
	HighAvailabilityNumberOfSeedServers int
	// HighAvailabilityNumberOfShootClients is the number of VPN shoot clients used for HA
	HighAvailabilityNumberOfShootClients int
	// PSPDisabled marks whether the PodSecurityPolicy admission plugin is disabled.
	PSPDisabled bool
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

type vpnSecret struct {
	name       string
	volumeName string
	mountPath  string
	secret     *corev1.Secret
}

func (v *vpnShoot) Deploy(ctx context.Context) error {
	var (
		config = &secretsutils.CertificateSecretConfig{
			Name:                        "vpn-shoot-client",
			CommonName:                  "vpn-shoot-client",
			CertType:                    secretsutils.ClientCert,
			SkipPublishingCACertificate: true,
		}
		signingCA = v1beta1constants.SecretNameCAVPN
	)

	secretCA, found := v.secretsManager.Get(signingCA)
	if !found {
		return fmt.Errorf("secret %q not found", signingCA)
	}

	var secrets []vpnSecret
	if !v.values.HighAvailabilityEnabled {
		secret, err := v.secretsManager.Generate(ctx, config, secretsmanager.SignedByCA(signingCA), secretsmanager.Rotate(secretsmanager.InPlace))
		if err != nil {
			return err
		}

		secrets = append(secrets, vpnSecret{
			name:       config.Name,
			volumeName: volumeName,
			mountPath:  volumeMountPathSecret,
			secret:     secret,
		})
	} else {
		for i := 0; i < v.values.HighAvailabilityNumberOfShootClients; i++ {
			config.Name = fmt.Sprintf("vpn-shoot-client-%d", i)
			config.CommonName = config.Name
			secret, err := v.secretsManager.Generate(ctx, config, secretsmanager.SignedByCA(signingCA), secretsmanager.Rotate(secretsmanager.InPlace))
			if err != nil {
				return err
			}
			secrets = append(secrets, vpnSecret{
				name:       config.Name,
				volumeName: fmt.Sprintf("%s-%d", volumeName, i),
				mountPath:  fmt.Sprintf("%s-%d", volumeMountPathSecret, i),
				secret:     secret,
			})
		}
	}

	data, err := v.computeResourcesData(secretCA, secrets)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, v.client, v.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
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

func (v *vpnShoot) computeResourcesData(secretCAVPN *corev1.Secret, secretsVPNShoot []vpnSecret) (map[string][]byte, error) {
	var (
		secretVPNSeedServerTLSAuth *corev1.Secret
		found                      bool
	)

	secretVPNSeedServerTLSAuth, found = v.secretsManager.Get(vpnseedserver.SecretNameTLSAuth)
	if !found {
		return nil, fmt.Errorf("secret %q not found", vpnseedserver.SecretNameTLSAuth)
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
		secretDH           *corev1.Secret
		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	utilruntime.Must(kubernetesutils.MakeUnique(secretCA))
	utilruntime.Must(kubernetesutils.MakeUnique(secretTLSAuth))

	for i, item := range secretsVPNShoot {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      item.name,
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: item.secret.Data,
		}
		utilruntime.Must(kubernetesutils.MakeUnique(secret))
		secretsVPNShoot[i].secret = secret
	}

	var (
		vpa *vpaautoscalingv1.VerticalPodAutoscaler

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-vpn",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: "Allows the VPN to communicate with shoot components and makes " +
						"the VPN reachable from the seed.",
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
		podSecurityPolicy *policyv1beta1.PodSecurityPolicy
		clusterRolePSP    *rbacv1.ClusterRole
		roleBindingPSP    *rbacv1.RoleBinding

		labels = map[string]string{
			v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
			v1beta1constants.LabelApp:       LabelValue,
			managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
		}
		template = v.podTemplate(serviceAccount, secretsVPNShoot, secretCA, secretTLSAuth)

		networkPolicyFromSeed = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-from-seed",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Ingress from the control plane to "+
						"pods labeled with '%s=%s'.", v1beta1constants.LabelNetworkPolicyShootFromSeed, v1beta1constants.LabelNetworkPolicyAllowed),
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyShootFromSeed: v1beta1constants.LabelNetworkPolicyAllowed}},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: template.Labels,
						},
					}},
				}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			},
		}
		deploymentOrStatefulSet client.Object
	)

	if !v.values.HighAvailabilityEnabled {
		deploymentOrStatefulSet = v.deployment(labels, template)
	} else {
		deploymentOrStatefulSet = v.statefulSet(labels, template)
	}

	utilruntime.Must(references.InjectAnnotations(deploymentOrStatefulSet))

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

	if v.values.VPAEnabled {
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		kind := "Deployment"
		if _, ok := deploymentOrStatefulSet.(*appsv1.StatefulSet); ok {
			kind = "StatefulSet"
		}
		containerNames := []string{containerName}
		if v.values.HighAvailabilityEnabled {
			containerNames = nil
			for i := 0; i < v.values.HighAvailabilityNumberOfSeedServers; i++ {
				containerNames = append(containerNames, fmt.Sprintf("%s-s%d", containerName, i))
			}
		}
		var containerPolicies []vpaautoscalingv1.ContainerResourcePolicy
		for _, name := range containerNames {
			containerPolicies = append(containerPolicies, vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName: name,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("10Mi"),
				},
				ControlledValues: &controlledValues,
			})
		}
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       kind,
					Name:       deploymentOrStatefulSet.GetName(),
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: containerPolicies,
				},
			},
		}
	}

	var objects []client.Object
	for _, item := range secretsVPNShoot {
		objects = append(objects, item.secret)
	}

	if v.values.HighAvailabilityEnabled {
		objects = append(objects, v.podDisruptionBudget())
	}

	objects = append(objects,
		secretCA,
		secretTLSAuth,
		secretDH,
		serviceAccount,
		networkPolicy,
		networkPolicyFromSeed,
		deploymentOrStatefulSet,
		clusterRole,
		clusterRoleBinding,
		vpa,
		podSecurityPolicy,
		clusterRolePSP,
		roleBindingPSP,
	)

	return registry.AddAllAndSerialize(objects...)
}

func (v *vpnShoot) podDisruptionBudget() client.Object {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: metav1.NamespaceSystem,
			Labels:    getLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: utils.IntStrPtrFromInt32(1),
			Selector:       &metav1.LabelSelector{MatchLabels: getLabels()},
		},
	}

	kubernetesutils.SetAlwaysAllowEviction(pdb, v.values.KubernetesVersion)

	return pdb
}

func (v *vpnShoot) podTemplate(serviceAccount *corev1.ServiceAccount, secrets []vpnSecret, secretCA, secretTLSAuth *corev1.Secret) *corev1.PodTemplateSpec {
	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
				v1beta1constants.LabelApp:       LabelValue,
				managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				"type":                          "tunnel",
			},
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           serviceAccount.Name,
			PriorityClassName:            "system-cluster-critical",
			DNSPolicy:                    corev1.DNSDefault,
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			InitContainers: v.getInitContainers(),
			Volumes:        v.getVolumes(secrets, secretCA, secretTLSAuth),
		},
	}

	if !v.values.HighAvailabilityEnabled {
		template.Spec.Containers = []corev1.Container{*v.container(secrets, nil)}
	} else {
		for i := 0; i < v.values.HighAvailabilityNumberOfSeedServers; i++ {
			template.Spec.Containers = append(template.Spec.Containers, *v.container(secrets, &i))
		}
	}

	return template
}

func (v *vpnShoot) container(secrets []vpnSecret, index *int) *corev1.Container {
	name := containerName
	if index != nil {
		name = fmt.Sprintf("%s-s%d", containerName, *index)
	}
	return &corev1.Container{
		Name:            name,
		Image:           v.values.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             v.getEnvVars(index),
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(false),
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
		VolumeMounts: v.getVolumeMounts(secrets),
	}
}

func (v *vpnShoot) deployment(labels map[string]string, template *corev1.PodTemplateSpec) *appsv1.Deployment {
	var (
		intStrMax  = intstr.FromString("100%")
		intStrZero = intstr.FromString("0%")
		replicas   = 1
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: metav1.NamespaceSystem,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			RevisionHistoryLimit: ptr.To(int32(2)),
			Replicas:             ptr.To(int32(replicas)),
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
			Template: *template,
		},
	}
}

func (v *vpnShoot) statefulSet(labels map[string]string, template *corev1.PodTemplateSpec) *appsv1.StatefulSet {
	replicas := v.values.HighAvailabilityNumberOfShootClients
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: metav1.NamespaceSystem,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			RevisionHistoryLimit: ptr.To(int32(2)),
			Replicas:             ptr.To(int32(replicas)),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: *template,
		},
	}
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
	return map[string]string{v1beta1constants.LabelApp: LabelValue}
}

func (v *vpnShoot) indexedReversedHeader(index *int) string {
	if index == nil {
		return v.values.ReversedVPN.Header
	}
	return strings.Replace(v.values.ReversedVPN.Header, "vpn-seed-server", fmt.Sprintf("vpn-seed-server-%d", *index), 1)
}

func (v *vpnShoot) getEnvVars(index *int) []corev1.EnvVar {
	var (
		envVariables []corev1.EnvVar
		ipFamilies   []string
	)
	for _, v := range v.values.ReversedVPN.IPFamilies {
		ipFamilies = append(ipFamilies, string(v))
	}
	envVariables = append(envVariables,
		corev1.EnvVar{
			Name:  "IP_FAMILIES",
			Value: strings.Join(ipFamilies, ","),
		},
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
			Value: v.indexedReversedHeader(index),
		},
		corev1.EnvVar{
			Name:  "DO_NOT_CONFIGURE_KERNEL_SETTINGS",
			Value: "true",
		},
		corev1.EnvVar{
			Name:  "IS_SHOOT_CLIENT",
			Value: "true",
		},
	)

	if index != nil {
		envVariables = append(envVariables,
			[]corev1.EnvVar{
				{
					Name:  "VPN_SERVER_INDEX",
					Value: fmt.Sprintf("%d", *index),
				},
				{
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
			}...)
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

func (v *vpnShoot) getVolumeMounts(secrets []vpnSecret) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{}
	for _, item := range secrets {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      item.volumeName,
			MountPath: item.mountPath,
		})
	}
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      volumeNameTLSAuth,
		MountPath: volumeMountPathSecretTLS,
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      volumeNameDevNetTun,
		MountPath: volumeMountPathDevNetTun,
	})

	return volumeMounts
}

func (v *vpnShoot) getVolumes(secret []vpnSecret, secretCA, secretTLSAuth *corev1.Secret) []corev1.Volume {
	volumes := []corev1.Volume{}
	for _, item := range secret {
		volumes = append(volumes, corev1.Volume{
			Name: item.volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To(int32(0400)),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretCA.Name,
								},
								Items: []corev1.KeyToPath{{
									Key:  secretsutils.DataKeyCertificateBundle,
									Path: "ca.crt",
								}},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: item.secret.Name,
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
		})
	}
	volumes = append(volumes, corev1.Volume{
		Name: volumeNameTLSAuth,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretTLSAuth.Name,
				DefaultMode: ptr.To(int32(0400)),
			},
		},
	})
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
	return volumes
}

func (v *vpnShoot) getInitContainers() []corev1.Container {
	container := corev1.Container{
		Name:            initContainerName,
		Image:           v.values.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name:  "IS_SHOOT_CLIENT",
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
				Name:  "EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS",
				Value: "true",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(true),
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
	}
	if v.values.HighAvailabilityEnabled {
		container.Env = append(container.Env, []corev1.EnvVar{
			{
				Name:  "CONFIGURE_BONDING",
				Value: "true",
			},
			{
				Name:  "HA_VPN_SERVERS",
				Value: fmt.Sprintf("%d", v.values.HighAvailabilityNumberOfSeedServers),
			},
			{
				Name:  "HA_VPN_CLIENTS",
				Value: fmt.Sprintf("%d", v.values.HighAvailabilityNumberOfShootClients),
			},
		}...)
	}
	return []corev1.Container{container}
}
