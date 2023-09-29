// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nginxingress

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	name = "nginx-ingress"
	// ManagedResourceName is the name of the nginx-ingress managed resource.
	ManagedResourceName = name
	// ManagedResourceNameShootAddon is the name of the nginx-ingress addon managed resource.
	ManagedResourceNameShootAddon = "shoot-addon-nginx-ingress"

	// LabelAppValue is the value of the 'app' label for the ingress controller.
	LabelAppValue = "nginx-ingress"
	// LabelKeyComponent is the 'component' key used in labels.
	LabelKeyComponent = "component"
	// LabelValueController is the value of the 'component' label for the ingress controller.
	LabelValueController = "controller"
	labelValueBackend    = "nginx-ingress-k8s-backend"
	labelKeyRelease      = "release"
	labelValueAddons     = "addons"

	controllerName          = "nginx-ingress-controller"
	containerNameController = controllerName
	backendName             = "nginx-ingress-k8s-backend"
	containerNameBackend    = backendName

	addonControllerName       = "addons-nginx-ingress-controller"
	addonName                 = "addons-nginx-ingress"
	addonBackendName          = "addons-nginx-ingress-nginx-ingress-k8s-backend"
	addonContainerNameBackend = "nginx-ingress-nginx-ingress-k8s-backend"

	roleBindingPSPName = "gardener.cloud:psp:addons-nginx-ingress"

	servicePortControllerHttp    int32 = 80
	containerPortControllerHttp  int32 = 80
	servicePortControllerHttps   int32 = 443
	containerPortControllerHttps int32 = 443

	servicePortBackend   int32 = 80
	containerPortBackend int32 = 8080
)

// Values is a set of configuration values for the nginx-ingress component.
type Values struct {
	// ClusterType specifies the type of the cluster to which nginx-ingress is being deployed.
	ClusterType component.ClusterType
	// TargetNamespace is the namespace in which the resources should be deployed
	TargetNamespace string
	// ImageController is the container image used for nginx-ingress controller.
	ImageController string
	// ImageDefaultBackend is the container image used for default ingress backend.
	ImageDefaultBackend string
	// IngressClass is the ingress class for the seed nginx-ingress controller
	IngressClass string
	// PriorityClassName is the name of the priority class for the nginx-ingress controller.
	PriorityClassName string
	// ConfigData contains the configuration details for the nginx-ingress controller
	ConfigData map[string]string
	// LoadBalancerAnnotations are the annotations added to the nginx-ingress load balancer service.
	LoadBalancerAnnotations map[string]string
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

	if n.values.ClusterType == component.ClusterTypeShoot {
		return managedresources.CreateForShoot(ctx, n.client, n.namespace, ManagedResourceNameShootAddon, managedresources.LabelValueGardener, false, data)
	}
	return managedresources.CreateForSeed(ctx, n.client, n.namespace, ManagedResourceName, false, data)
}

func (n *nginxIngress) Destroy(ctx context.Context) error {
	if n.values.ClusterType == component.ClusterTypeShoot {
		return managedresources.DeleteForShoot(ctx, n.client, n.namespace, ManagedResourceNameShootAddon)
	}
	return managedresources.DeleteForSeed(ctx, n.client, n.namespace, ManagedResourceName)
}

var (
	// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
	// or deleted.
	TimeoutWaitForManagedResource = 2 * time.Minute
	// WaitUntilHealthy is an alias for managedresources.WaitUntilHealthy. Exposed for tests.
	WaitUntilHealthy = managedresources.WaitUntilHealthy
)

func (n *nginxIngress) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	name := ManagedResourceName
	if n.values.ClusterType == component.ClusterTypeShoot {
		name = ManagedResourceNameShootAddon
	}

	return WaitUntilHealthy(timeoutCtx, n.client, n.namespace, name)
}

func (n *nginxIngress) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	name := ManagedResourceName
	if n.values.ClusterType == component.ClusterTypeShoot {
		name = ManagedResourceNameShootAddon
	}

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, name)
}

func (n *nginxIngress) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.getName("ConfigMap", false),
				Labels:    n.getLabels(LabelValueController, false),
				Namespace: n.values.TargetNamespace,
			},
			Data: n.values.ConfigData,
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.getName("ServiceAccount", false),
				Namespace: n.values.TargetNamespace,
				Labels:    map[string]string{v1beta1constants.LabelApp: LabelAppValue},
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		serviceAnnotations                      = n.values.LoadBalancerAnnotations
		initialDelaySecondsLivenessProbe  int32 = 40
		initialDelaySecondsReadinessProbe int32
		nodeSelector                      map[string]string
		roleBindingAnnotations            map[string]string
		schedulerName                     string

		intStrOne            = intstr.FromInt32(1)
		healthProbePort      = intstr.FromInt32(10254)
		resourceRequirements = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1500Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("375Mi"),
			},
		}
	)

	if n.values.ClusterType == component.ClusterTypeSeed {
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		initialDelaySecondsReadinessProbe = 40

		// We don't call kubernetesutils.MakeUnique() here if the cluster is shoot, because extensions might need
		// to mutate it and the name is hard-coded. See https://github.com/gardener/gardener/pull/7386 for more details.
		utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	}

	if n.values.ClusterType == component.ClusterTypeShoot {
		serviceAccount.Labels = utils.MergeStringMaps[string](serviceAccount.Labels, map[string]string{
			labelKeyRelease: labelValueAddons,
		})
		serviceAnnotations = map[string]string{"service.beta.kubernetes.io/aws-load-balancer-proxy-protocol": "*"}
		nodeSelector = map[string]string{v1beta1constants.LabelWorkerPoolSystemComponents: "true"}
		roleBindingAnnotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
		schedulerName = corev1.DefaultSchedulerName
		initialDelaySecondsLivenessProbe = 10

		resourceRequirements = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		}
	}

	var (
		serviceController = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        n.getName("Service", false),
				Namespace:   n.values.TargetNamespace,
				Labels:      n.getLabels(LabelValueController, false),
				Annotations: serviceAnnotations,
			},
			Spec: corev1.ServiceSpec{
				Type:                     corev1.ServiceTypeLoadBalancer,
				LoadBalancerSourceRanges: n.values.LoadBalancerSourceRanges,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       servicePortControllerHttp,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(containerPortControllerHttp),
					},
					{
						Name:       "https",
						Port:       servicePortControllerHttps,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(containerPortControllerHttps),
					},
				},
				Selector: n.getLabels(LabelValueController, false),
			},
		}

		serviceBackend = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.getName("Service", true),
				Labels:    n.getLabels(labelValueBackend, false),
				Namespace: n.values.TargetNamespace,
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{
					Port:       servicePortBackend,
					TargetPort: intstr.FromInt32(containerPortBackend),
				}},
				Selector: n.getLabels(labelValueBackend, false),
			},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.getName("Role", false),
				Namespace: n.values.TargetNamespace,
				Labels:    n.getLabels("", false),
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
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{n.getName("Lease", false)},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"discovery.k8s.io"},
					Resources: []string{"endpointslices"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        n.getName("RoleBinding", false),
				Namespace:   n.values.TargetNamespace,
				Labels:      n.getLabels("", false),
				Annotations: roleBindingAnnotations,
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

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   n.getName("ClusterRole", false),
				Labels: n.getLabels("", false),
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
					APIGroups: []string{"networking.k8s.io"},
					Resources: []string{"ingresses"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{"networking.k8s.io"},
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
				{
					APIGroups: []string{"discovery.k8s.io"},
					Resources: []string{"endpointslices"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        n.getName("ClusterRoleBinding", false),
				Labels:      n.getLabels("", false),
				Annotations: roleBindingAnnotations,
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
				Name:      n.getName("Deployment", true),
				Namespace: n.values.TargetNamespace,
				Labels:    n.getLabels(labelValueBackend, true),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: utils.MergeStringMaps(n.getLabels(labelValueBackend, false), map[string]string{
						labelKeyRelease: labelValueAddons,
					}),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(n.getLabels(labelValueBackend, true), map[string]string{
							labelKeyRelease: labelValueAddons,
						}),
					},
					Spec: corev1.PodSpec{
						PriorityClassName: n.values.PriorityClassName,
						NodeSelector:      nodeSelector,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser: pointer.Int64(65534),
							FSGroup:   pointer.Int64(65534),
						},
						Containers: []corev1.Container{{
							Name:            n.getName("Container", true),
							Image:           n.values.ImageDefaultBackend,
							ImagePullPolicy: corev1.PullIfNotPresent,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthy",
										Port:   intstr.FromInt32(containerPortBackend),
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
						TerminationGracePeriodSeconds: pointer.Int64(60),
					},
				},
			},
		}

		deploymentController = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.getName("Deployment", false),
				Namespace: n.values.TargetNamespace,
				Labels:    n.getLabels(LabelValueController, true),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(2),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: utils.MergeStringMaps[string](n.getLabels(LabelValueController, false), map[string]string{
						labelKeyRelease: labelValueAddons,
					}),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps[string](n.getLabels(LabelValueController, true), map[string]string{
							labelKeyRelease: labelValueAddons,
						}),
					},
					Spec: corev1.PodSpec{
						PriorityClassName: n.values.PriorityClassName,
						SchedulerName:     schedulerName,
						NodeSelector:      nodeSelector,
						Containers: []corev1.Container{{
							Name:            n.getName("Container", false),
							Image:           n.values.ImageController,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            n.getArgs(configMap.Name, serviceController.Name),
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
									Add:  []corev1.Capability{"NET_BIND_SERVICE", "SYS_CHROOT"},
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
								InitialDelaySeconds: initialDelaySecondsLivenessProbe,
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
								FailureThreshold:    3,
								InitialDelaySeconds: initialDelaySecondsReadinessProbe,
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
							Resources: resourceRequirements,
						}},
						ServiceAccountName:            serviceAccount.Name,
						TerminationGracePeriodSeconds: pointer.Int64(60),
					},
				},
			},
		}

		ingressClass = &networkingv1.IngressClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:   n.values.IngressClass,
				Labels: n.getLabels(LabelValueController, true),
			},
			Spec: networkingv1.IngressClassSpec{
				Controller: "k8s.io/" + n.values.IngressClass,
			},
		}

		updateMode          = vpaautoscalingv1.UpdateModeAuto
		vpa                 *vpaautoscalingv1.VerticalPodAutoscaler
		roleBindingPSP      *rbacv1.RoleBinding
		podDisruptionBudget *policyv1.PodDisruptionBudget
		networkPolicy       *networkingv1.NetworkPolicy
	)

	if n.values.ClusterType == component.ClusterTypeSeed {
		metav1.SetMetaDataAnnotation(&serviceController.ObjectMeta, resourcesv1alpha1.NetworkingFromWorldToPorts, fmt.Sprintf(`[{"protocol":"TCP","port":%d},{"protocol":"TCP","port":%d}]`, containerPortControllerHttp, containerPortControllerHttps))
		metav1.SetMetaDataAnnotation(&deploymentController.ObjectMeta, references.AnnotationKey(references.KindConfigMap, configMap.Name), configMap.Name)

		deploymentController.Spec.Template.Annotations = map[string]string{
			// InjectAnnotations function is not used here since the ConfigMap is not mounted as
			// volume and hence using the function won't have any effect.
			references.AnnotationKey(references.KindConfigMap, configMap.Name): configMap.Name,
		}
		deploymentController.Spec.Template.Labels = utils.MergeStringMaps(deploymentController.Spec.Template.Labels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:                                           v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                              v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel(n.getName("Service", true), containerPortBackend): v1beta1constants.LabelNetworkPolicyAllowed,

			// Skipped until https://github.com/kubernetes/ingress-nginx/issues/8640 is resolved
			// and special seccomp profile is implemented for the nginx-ingress
			resourcesv1alpha1.SeccompProfileSkip: "true",
		})

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      controllerName,
				Namespace: n.values.TargetNamespace,
				Labels:    n.getLabels(LabelValueController, false),
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &intStrOne,
				Selector: &metav1.LabelSelector{
					MatchLabels: n.getLabels(LabelValueController, false),
				},
			},
		}
	}

	if n.values.ClusterType == component.ClusterTypeShoot {
		serviceController.Spec.ExternalTrafficPolicy = n.values.ExternalTrafficPolicy

		deploymentBackend.Spec.Template.Spec.SecurityContext.SupplementalGroups = []int64{1}
		deploymentBackend.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}

		deploymentController.Spec.Replicas = pointer.Int32(1)
		deploymentController.Spec.Template.Annotations = map[string]string{"checksum/config": utils.ComputeChecksum(configMap.Data)}
		deploymentController.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirst
		deploymentController.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyAlways
		deploymentController.Spec.Template.Spec.Containers[0].TerminationMessagePath = corev1.TerminationMessagePathDefault
		deploymentController.Spec.Template.Spec.Containers[0].TerminationMessagePolicy = corev1.TerminationMessageReadFile

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-to-from-nginx",
				Namespace: n.values.TargetNamespace,
				Annotations: map[string]string{v1beta1constants.GardenerDescription: "Allows all egress and ingress " +
					"traffic for the nginx-ingress controller.",
				},
				Labels: map[string]string{
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: *deploymentController.Spec.Selector,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
				Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
			},
		}
	}

	if n.values.VPAEnabled {
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.getName("VPA", false),
				Namespace: n.values.TargetNamespace,
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
								corev1.ResourceMemory: resource.MustParse("100Mi"),
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
				Name:     "gardener.cloud:psp:privileged",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
	}

	return registry.AddAllAndSerialize(
		clusterRole,
		clusterRoleBinding,
		serviceAccount,
		configMap,
		serviceController,
		deploymentController,
		podDisruptionBudget,
		vpa,
		role,
		roleBinding,
		serviceBackend,
		deploymentBackend,
		ingressClass,
		roleBindingPSP,
		networkPolicy,
	)
}

func (n *nginxIngress) getLabels(componentLabelValue string, optionalLabels bool) map[string]string {
	labels := map[string]string{
		v1beta1constants.LabelApp: LabelAppValue,
	}
	if componentLabelValue != "" {
		labels[LabelKeyComponent] = componentLabelValue
	}
	if n.values.ClusterType == component.ClusterTypeShoot {
		labels[labelKeyRelease] = labelValueAddons
		if optionalLabels {
			labels[v1beta1constants.GardenRole] = v1beta1constants.GardenRoleOptionalAddon
			labels[managedresources.LabelKeyOrigin] = managedresources.LabelValueGardener
		}
	}
	return labels
}

func (n *nginxIngress) getArgs(configMapName, serviceNameController string) []string {
	out := []string{
		"/nginx-ingress-controller",
		"--default-backend-service=" + n.values.TargetNamespace + "/" + n.getName("Service", true),
		"--enable-ssl-passthrough=true",
		"--publish-service=" + n.values.TargetNamespace + "/" + serviceNameController,
		"--election-id=" + n.getName("Lease", false),
		"--update-status=true",
		"--annotations-prefix=nginx.ingress.kubernetes.io",
		"--configmap=" + n.values.TargetNamespace + "/" + configMapName,
		"--ingress-class=" + n.values.IngressClass,
		"--controller-class=k8s.io/" + n.values.IngressClass,
	}

	if n.values.ClusterType == component.ClusterTypeShoot {
		out = append(out, "--watch-ingress-without-class=true")
	}

	return out
}

func (n *nginxIngress) getName(kind string, backend bool) string {
	switch kind {
	case "Deployment", "Service", "ConfigMap", "VPA":
		if backend {
			if n.values.ClusterType == component.ClusterTypeShoot {
				return addonBackendName
			}
			return backendName
		}
		if n.values.ClusterType == component.ClusterTypeShoot {
			return addonControllerName
		}
		return controllerName
	case "ServiceAccount":
		if n.values.ClusterType == component.ClusterTypeShoot {
			return addonName
		}
		return name
	case "Container":
		if backend {
			if n.values.ClusterType == component.ClusterTypeShoot {
				return addonContainerNameBackend
			}
			return containerNameBackend
		}
		return containerNameController
	case "ClusterRole", "ClusterRoleBinding", "Role", "RoleBinding":
		if n.values.ClusterType == component.ClusterTypeShoot {
			return addonName
		}
		if kind == "Role" {
			return "gardener.cloud:seed:" + name + ":role"
		}
		if kind == "RoleBinding" {
			return "gardener.cloud:seed:" + name + ":role-binding"
		}
		return "gardener.cloud:seed:" + name
	case "Lease":
		if n.values.ClusterType == component.ClusterTypeShoot {
			return "ingress-controller-leader"
		}
		return "ingress-controller-seed-leader"
	}

	return ""
}
