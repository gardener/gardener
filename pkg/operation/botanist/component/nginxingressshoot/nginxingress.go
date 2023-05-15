// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nginxingressshoot

import (
	"context"
	"time"

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	// ManagedResourceName is the name of the nginx-ingress addon managed resource.
	ManagedResourceName = "shoot-addon-nginx-ingress"

	labelAppValue        = "nginx-ingress"
	labelKeyComponent    = "component"
	labelValueController = "controller"
	labelKeyRelease      = "release"
	labelValueAddons     = "addons"
	labelValueBackend    = "nginx-ingress-k8s-backend"

	clusterRoleName          = "addons-nginx-ingress"
	serviceAccountName       = "addons-nginx-ingress"
	clusterRoleBindingName   = "addons-nginx-ingress"
	clusterRolePSPName       = "gardener.cloud:psp:privileged"
	configMapName            = "addons-nginx-ingress-controller"
	deploymentNameController = "addons-nginx-ingress-controller"
	ingressClassName         = "nginx"
	roleName                 = "addons-nginx-ingress"
	roleBindingName          = "addons-nginx-ingress"
	roleBindingPSPName       = "gardener.cloud:psp:addons-nginx-ingress"
	serviceNameController    = "addons-nginx-ingress-controller"
	containerNameController  = "nginx-ingress-controller"
	containerNameBackend     = "nginx-ingress-nginx-ingress-k8s-backend"
	serviceNameBackend       = "addons-nginx-ingress-nginx-ingress-k8s-backend"
	deploymentNameBackend    = "addons-nginx-ingress-nginx-ingress-k8s-backend"
	vpaName                  = "addons-nginx-ingress-controller"

	servicePortControllerHttp    int32 = 80
	containerPortControllerHttp  int32 = 80
	servicePortControllerHttps   int32 = 443
	containerPortControllerHttps int32 = 443
	containerPortBackend         int32 = 8080
	servicePortBackend           int32 = 80
)

// Values is a set of configuration values for the nginx-ingress component.
type Values struct {
	// NginxControllerImage is the container image used for nginx-ingress controller.
	NginxControllerImage string
	// DefaultBackendImage is the container image used for default ingress backend.
	DefaultBackendImage string
	// KubernetesVersion is the kubernetes version of the shoot.
	KubernetesVersion *semver.Version
	// ConfigData contains the configuration details for the nginx-ingress controller.
	ConfigData map[string]string
	// KubeAPIServerHost is the host of the kube-apiserver.
	KubeAPIServerHost *string
	// LoadBalancerSourceRanges is list of allowed IP sources for NginxIngress.
	LoadBalancerSourceRanges []string
	// ExternalTrafficPolicy controls the `.spec.externalTrafficPolicy` value of the load balancer `Service`
	// exposing the nginx-ingress.
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicyType
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// PSPDisabled marks whether the PodSecurityPolicy admission plugin is disabled.
	PSPDisabled bool
}

// New creates a new instance of DeployWaiter for nginx-ingress
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &nginxIngress{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nginxIngress struct {
	client    client.Client
	namespace string
	values    Values
}

func (n *nginxIngress) Deploy(ctx context.Context) error {
	data, err := n.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, n.client, n.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
}

func (n *nginxIngress) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, n.client, n.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (n *nginxIngress) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, n.client, n.namespace, ManagedResourceName)
}

func (n *nginxIngress) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, ManagedResourceName)
}

func (n *nginxIngress) computeResourcesData() (map[string][]byte, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
			Labels: map[string]string{
				v1beta1constants.LabelApp: labelAppValue,
				labelKeyRelease:           labelValueAddons,
				labelKeyComponent:         labelValueController,
			},
			Namespace: metav1.NamespaceSystem,
		},
		Data: n.values.ConfigData,
	}

	// We don't call kubernetesutils.MakeUnique() here because the configmap needs to be mutable, since the nginx controller
	// mutates it in some cases. See https://github.com/gardener/gardener/pull/7386 for more details.

	var (
		healthProbePort = intstr.FromInt(10254)

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp:         labelAppValue,
					labelKeyRelease:                   labelValueAddons,
					"addonmanager.kubernetes.io/mode": "Reconcile",
				},
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleName,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyRelease:           labelValueAddons,
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints", "nodes", "pods", "secrets"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"extensions", "networking.k8s.io"},
					Resources: []string{"ingresses"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{"extensions", "networking.k8s.io"},
					Resources: []string{"ingresses/status"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"networking.k8s.io"},
					Resources: []string{"ingressclasses"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleBindingName,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyRelease:           labelValueAddons,
				},
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		deploymentBackend = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentNameBackend,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp:   labelAppValue,
					labelKeyComponent:           labelValueBackend,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleOptionalAddon,
					labelKeyRelease:             labelValueAddons,
					"origin":                    "gardener",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.LabelApp: labelAppValue,
						labelKeyComponent:         labelValueBackend,
						labelKeyRelease:           labelValueAddons,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1beta1constants.LabelApp:   labelAppValue,
							labelKeyComponent:           labelValueBackend,
							v1beta1constants.GardenRole: v1beta1constants.GardenRoleOptionalAddon,
							labelKeyRelease:             labelValueAddons,
							"origin":                    "gardener",
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:             v1beta1constants.PriorityClassNameShootSystem600,
						NodeSelector:                  map[string]string{v1beta1constants.LabelWorkerPoolSystemComponents: "true"},
						TerminationGracePeriodSeconds: pointer.Int64(60),
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:          pointer.Int64(65534),
							FSGroup:            pointer.Int64(65534),
							SupplementalGroups: []int64{1},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{{
							Name:            containerNameBackend,
							Image:           n.values.DefaultBackendImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthy",
										Port:   intstr.FromInt(int(containerPortBackend)),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 30,
								TimeoutSeconds:      5,
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: containerPortBackend,
								Protocol:      corev1.ProtocolTCP,
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
						}},
					},
				},
			},
		}

		deploymentController = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentNameController,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleOptionalAddon,
					v1beta1constants.LabelApp:   labelAppValue,
					labelKeyRelease:             labelValueAddons,
					labelKeyComponent:           labelValueController,
					"origin":                    "gardener",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.LabelApp: labelAppValue,
						labelKeyRelease:           labelValueAddons,
						labelKeyComponent:         labelValueController,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1beta1constants.GardenRole: v1beta1constants.GardenRoleOptionalAddon,
							v1beta1constants.LabelApp:   labelAppValue,
							labelKeyRelease:             labelValueAddons,
							labelKeyComponent:           labelValueController,
							"origin":                    "gardener",
						},
						Annotations: map[string]string{
							"checksum/config": utils.ComputeChecksum(configMap.Data),
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:             v1beta1constants.PriorityClassNameShootSystem600,
						DNSPolicy:                     corev1.DNSClusterFirst,
						RestartPolicy:                 corev1.RestartPolicyAlways,
						SchedulerName:                 corev1.DefaultSchedulerName,
						ServiceAccountName:            serviceAccount.Name,
						TerminationGracePeriodSeconds: pointer.Int64(60),
						NodeSelector:                  map[string]string{v1beta1constants.LabelWorkerPoolSystemComponents: "true"},
						Containers: []corev1.Container{{
							Name:                     containerNameController,
							Image:                    n.values.NginxControllerImage,
							ImagePullPolicy:          corev1.PullIfNotPresent,
							Args:                     n.getArgs(configMap.Name),
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
									Add:  []corev1.Capability{"NET_BIND_SERVICE"},
								},
								RunAsUser:                pointer.Int64(101),
								AllowPrivilegeEscalation: pointer.Bool(true),
								SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   healthProbePort,
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      1,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: containerPortControllerHttp,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "https",
									ContainerPort: containerPortControllerHttps,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   healthProbePort,
										Scheme: corev1.URISchemeHTTP,
									},
								},
								PeriodSeconds:    10,
								SuccessThreshold: 1,
								TimeoutSeconds:   1,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("4Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
						}},
					},
				},
			},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyRelease:           labelValueAddons,
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps", "namespaces", "pods", "secrets"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints"},
					Verbs:     []string{"create", "get", "update"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"ingress-controller-leader"},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"ingress-controller-leader"},
					Verbs:         []string{"get", "update"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyRelease:           labelValueAddons,
				},
				Annotations: map[string]string{
					resourcesv1alpha1.DeleteOnInvalidUpdate: "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-to-from-nginx",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{v1beta1constants.GardenerDescription: "Allows all egress and ingress " +
					"traffic for the nginx-ingress controller.",
				},
				Labels: map[string]string{
					"origin": "gardener",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: *deploymentController.Spec.Selector,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
				Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
			},
		}

		serviceController = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceNameController,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyRelease:           labelValueAddons,
					labelKeyComponent:         labelValueController,
				},
				Annotations: map[string]string{"service.beta.kubernetes.io/aws-load-balancer-proxy-protocol": "*"},
			},
			Spec: corev1.ServiceSpec{
				Type:                     corev1.ServiceTypeLoadBalancer,
				LoadBalancerSourceRanges: n.values.LoadBalancerSourceRanges,
				ExternalTrafficPolicy:    n.values.ExternalTrafficPolicy,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       servicePortControllerHttp,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(int(containerPortControllerHttp)),
					},
					{
						Name:       "https",
						Port:       servicePortControllerHttps,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(int(containerPortControllerHttps)),
					},
				},
				Selector: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyRelease:           labelValueAddons,
					labelKeyComponent:         labelValueController,
				},
			},
		}

		serviceBackend = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceNameBackend,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyComponent:         labelValueBackend,
					labelKeyRelease:           labelValueAddons,
				},
				Namespace: metav1.NamespaceSystem,
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{
					Port:       servicePortBackend,
					TargetPort: intstr.FromInt(int(containerPortBackend)),
				}},
				Selector: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyComponent:         labelValueBackend,
					labelKeyRelease:           labelValueAddons,
				},
			},
		}

		ingressClass   *networkingv1.IngressClass
		vpa            *vpaautoscalingv1.VerticalPodAutoscaler
		roleBindingPSP *rbacv1.RoleBinding
	)

	if n.values.VPAEnabled {
		updateMode := vpaautoscalingv1.UpdateModeAuto
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vpaName,
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deploymentController.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &updateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
			},
		}
	}

	if !n.values.PSPDisabled {
		roleBindingPSP = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingPSPName,
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					resourcesv1alpha1.DeleteOnInvalidUpdate: "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRolePSPName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
	}

	if version.ConstraintK8sGreaterEqual122.Check(n.values.KubernetesVersion) {
		ingressClass = &networkingv1.IngressClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: ingressClassName,
				Labels: map[string]string{
					v1beta1constants.LabelApp:   labelAppValue,
					labelKeyComponent:           labelValueController,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleOptionalAddon,
					labelKeyRelease:             labelValueAddons,
					"origin":                    "gardener",
				},
			},
			Spec: networkingv1.IngressClassSpec{
				Controller: "k8s.io/nginx",
			},
		}

		deploymentController.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Add = append(deploymentController.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Add, "SYS_CHROOT")
	}

	return registry.AddAllAndSerialize(
		clusterRole,
		clusterRoleBinding,
		configMap,
		deploymentController,
		deploymentBackend,
		ingressClass,
		role,
		roleBinding,
		roleBindingPSP,
		serviceAccount,
		networkPolicy,
		serviceController,
		serviceBackend,
		vpa,
	)
}

func (n *nginxIngress) getArgs(configMapName string) []string {
	out := []string{
		"/nginx-ingress-controller",
		"--default-backend-service=" + metav1.NamespaceSystem + "/" + serviceNameBackend,
		"--enable-ssl-passthrough=true",
		"--publish-service=" + metav1.NamespaceSystem + "/" + serviceNameController,
		"--election-id=ingress-controller-leader",
		"--update-status=true",
		"--annotations-prefix=nginx.ingress.kubernetes.io",
		"--ingress-class=nginx",
		"--configmap=" + metav1.NamespaceSystem + "/" + configMapName,
	}

	if version.ConstraintK8sGreaterEqual122.Check(n.values.KubernetesVersion) {
		out = append(out, "--controller-class=k8s.io/nginx")
		out = append(out, "--watch-ingress-without-class=true")
	}

	return out
}
