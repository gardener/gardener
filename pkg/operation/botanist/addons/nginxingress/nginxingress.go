// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	clusterRoleName          = "addons-nginx-ingress"
	serviceAccountName       = "addons-nginx-ingress"
	clusterRoleBindingName   = "addons-nginx-ingress"
	configMapName            = "addons-nginx-ingress-controller"
	deploymentNameController = "addons-nginx-ingress-controller"
	ingressClassName         = "nginx"
	serviceNameController    = "addons-nginx-ingress-controller"
	containerNameController  = "nginx-ingress-controller"
	serviceNameBackend       = "addons-nginx-ingress-nginx-ingress-k8s-backend"

	servicePortControllerHttp    int32 = 80
	containerPortControllerHttp  int32 = 80
	servicePortControllerHttps   int32 = 443
	containerPortControllerHttps int32 = 443
)

// Values is a set of configuration values for the nginx-ingress component.
type Values struct {
	// ImageController is the container image used for nginx-ingress controller.
	ImageController string
	// ImageDefaultBackend is the container image used for default ingress backend.
	ImageDefaultBackend string
	// KubernetesVersion is the version of kubernetes for the shoot cluster.
	KubernetesVersion *semver.Version
	// ConfigData contains the configuration details for the nginx-ingress controller
	ConfigData map[string]string
	// KubeAPIServerHost is the host of the kube-apiserver.
	KubeAPIServerHost string
	// LoadBalancerSourceRanges is list of allowed IP sources for NginxIngress.
	LoadBalancerSourceRanges []string
	// ExternalTrafficPolicy controls the `.spec.externalTrafficPolicy` value of the load balancer `Service`
	// exposing the nginx-ingress.
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicyType
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
	var (
		healthProbePort = intstr.FromInt(10254)

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: configMapName,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					labelKeyRelease:           labelValueAddons,
					labelKeyComponent:         labelValueController,
				},
				Namespace: n.namespace,
			},
			Data: n.values.ConfigData,
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: n.namespace,
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

		deploymentController = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentNameController,
				Namespace: n.namespace,
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
				RevisionHistoryLimit: pointer.Int32(1),
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
						PriorityClassName:             "system-cluster-critical",
						DNSPolicy:                     corev1.DNSClusterFirst,
						RestartPolicy:                 corev1.RestartPolicyAlways,
						SchedulerName:                 corev1.DefaultSchedulerName,
						ServiceAccountName:            serviceAccount.Name,
						TerminationGracePeriodSeconds: pointer.Int64(60),
						NodeSelector:                  map[string]string{v1beta1constants.LabelWorkerPoolSystemComponents: "true"},
						Containers: []corev1.Container{{
							Name:                     containerNameController,
							Image:                    n.values.ImageController,
							ImagePullPolicy:          corev1.PullIfNotPresent,
							Args:                     n.getArgs(configMap),
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

		serviceController = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceNameController,
				Namespace: n.namespace,
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

		ingressClass *networkingv1.IngressClass
	)

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
		ingressClass,
		serviceAccount,
		serviceController,
	)
}

func (n *nginxIngress) getArgs(configMap *corev1.ConfigMap) []string {
	out := []string{
		"/nginx-ingress-controller",
		"--default-backend-service=" + n.namespace + "/" + serviceNameBackend,
		"--enable-ssl-passthrough=true",
		"--publish-service=" + n.namespace + "/" + serviceNameController,
		"--election-id=ingress-controller-seed-leader",
		"--update-status=true",
		"--annotations-prefix=nginx.ingress.kubernetes.io",
		"--ingress-class=nginx",
		"--configmap=" + n.namespace + "/" + configMap.Name,
	}

	if version.ConstraintK8sGreaterEqual122.Check(n.values.KubernetesVersion) {
		out = append(out, "--controller-class=k8s.io/nginx")
		out = append(out, "--watch-ingress-without-class=true")
	}

	return out
}
