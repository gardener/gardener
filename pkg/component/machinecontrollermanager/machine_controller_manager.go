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

package machinecontrollermanager

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	portMetrics               = 10258
	portNameMetrics           = "metrics"
	containerName             = "machine-controller-manager"
	serviceName               = "machine-controller-manager"
	managedResourceTargetName = "shoot-core-machine-controller-manager"
	// VPAName is the name of the vertical pod autoscaler for the machine-controller-manager.
	VPAName = "machine-controller-manager-vpa"
)

// Interface contains functions for a machine-controller-manager deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetNamespaceUID sets the UID of the namespace into which the cluster-autoscaler shall be deployed.
	SetNamespaceUID(types.UID)
}

// New creates a new instance of DeployWaiter for the machine-controller-manager.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &machineControllerManager{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type machineControllerManager struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// Values is a set of configuration values for the machine-controller-manager component.
type Values struct {
	// Image is the container image used for machine-controller-manager.
	Image string
	// Replicas is the number of replicas for the deployment.
	Replicas int32
	// RuntimeKubernetesVersion is the Kubernetes version of the runtime cluster.
	RuntimeKubernetesVersion *semver.Version

	namespaceUID types.UID
}

func (m *machineControllerManager) Deploy(ctx context.Context) error {
	var (
		shootAccessSecret   = m.newShootAccessSecret()
		serviceAccount      = m.emptyServiceAccount()
		clusterRoleBinding  = m.emptyClusterRoleBindingRuntime()
		service             = m.emptyService()
		deployment          = m.emptyDeployment()
		podDisruptionBudget = m.emptyPodDisruptionBudget()
		vpa                 = m.emptyVPA()

		vpaUpdateMode       = vpaautoscalingv1.UpdateModeAuto
		vpaControlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	)

	genericTokenKubeconfigSecret, found := m.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, m.client, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, m.client, clusterRoleBinding, func() error {
		clusterRoleBinding.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         "v1",
			Kind:               "Namespace",
			Name:               m.namespace,
			UID:                m.values.namespaceUID,
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		}}
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccount.Name,
			Namespace: m.namespace,
		}}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, service, func() error {
		service.Labels = utils.MergeStringMaps(service.Labels, getLabels())

		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkingv1.NetworkPolicyPort{
			Port:     utils.IntStrPtrFromInt32(portMetrics),
			Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
		}))

		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		desiredPorts := []corev1.ServicePort{{
			Name:     portNameMetrics,
			Protocol: corev1.ProtocolTCP,
			Port:     portMetrics,
		}}
		service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, m.client); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, getLabels(), map[string]string{
			v1beta1constants.GardenRole:                  v1beta1constants.GardenRoleControlPlane,
			resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
		})
		deployment.Spec.Replicas = &m.values.Replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32(2)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                                                                                 v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart:                                                                 "true",
					v1beta1constants.LabelNetworkPolicyToDNS:                                                                    v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToPublicNetworks:                                                         v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToPrivateNetworks:                                                        v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                                                       v1beta1constants.LabelNetworkPolicyAllowed,
					gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            containerName,
					Image:           m.values.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"./machine-controller-manager",
						"--control-kubeconfig=inClusterConfig",
						"--machine-safety-overshooting-period=1m",
						"--namespace=" + m.namespace,
						fmt.Sprintf("--port=%d", portMetrics),
						"--safety-up=2",
						"--safety-down=1",
						"--target-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
						"--v=3",
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/healthz",
								Port:   intstr.FromInt32(portMetrics),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						FailureThreshold:    3,
						InitialDelaySeconds: 30,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						TimeoutSeconds:      5,
					},
					Ports: []corev1.ContainerPort{{
						Name:          portNameMetrics,
						ContainerPort: portMetrics,
						Protocol:      corev1.ProtocolTCP,
					}},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("31m"),
							corev1.ResourceMemory: resource.MustParse("70Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1024Mi"),
						},
					}},
				},
				PriorityClassName:             v1beta1constants.PriorityClassNameShootControlPlane300,
				ServiceAccountName:            serviceAccount.Name,
				TerminationGracePeriodSeconds: pointer.Int64(5),
			},
		}

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, podDisruptionBudget, func() error {
		podDisruptionBudget.Labels = utils.MergeStringMaps(podDisruptionBudget.Labels, getLabels())
		podDisruptionBudget.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: utils.IntStrPtrFromInt32(1),
			Selector:       deployment.Spec.Selector,
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       deployment.Name,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &vpaUpdateMode}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName:    containerName,
				ControlledValues: &vpaControlledValues,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("70Mi"),
				},
				MaxAllowed: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("5G"),
				},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	data, err := m.computeShootResourcesData(shootAccessSecret.ServiceAccountName)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, m.client, m.namespace, managedResourceTargetName, managedresources.LabelValueGardener, false, data)
}

func (m *machineControllerManager) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, m.client,
		m.emptyManagedResource(),
		m.emptyManagedResourceSecret(),
		m.emptyVPA(),
		m.emptyPodDisruptionBudget(),
		m.emptyDeployment(),
		m.newShootAccessSecret().Secret,
		m.emptyService(),
		m.emptyClusterRoleBindingRuntime(),
		m.emptyServiceAccount(),
	)
}

var (
	// DefaultInterval is the default interval.
	DefaultInterval = 5 * time.Second
	// DefaultTimeout is the default timeout.
	DefaultTimeout = 5 * time.Minute
)

func (m *machineControllerManager) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	deployment := m.emptyDeployment()
	return retry.Until(timeoutCtx, DefaultInterval, health.IsDeploymentUpdated(m.client, deployment))
}

func (m *machineControllerManager) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	return retry.Until(timeoutCtx, DefaultInterval, func(ctx context.Context) (done bool, err error) {
		deploy := m.emptyDeployment()
		err = m.client.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)
		switch {
		case apierrors.IsNotFound(err):
			return retry.Ok()
		case err == nil:
			return retry.MinorError(err)
		default:
			return retry.SevereError(err)
		}
	})
}

func (m *machineControllerManager) SetNamespaceUID(uid types.UID) { m.values.namespaceUID = uid }

func (m *machineControllerManager) computeShootResourcesData(serviceAccountName string) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:machine-controller-manager",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes", "nodes/status", "endpoints", "replicationcontrollers", "pods", "persistentvolumes", "persistentvolumeclaims"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/eviction"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"replicasets", "statefulsets", "daemonsets", "deployments"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs", "cronjobs"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"policy"},
					Resources: []string{"poddisruptionbudgets"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"storage.k8s.io"},
					Resources: []string{"volumeattachments"},
					Verbs:     []string{"delete", "get", "list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:machine-controller-manager",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		role = &rbacv1.Role{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:machine-controller-manager",
				Namespace: metav1.NamespaceSystem,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"create", "delete", "get", "list"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:machine-controller-manager",
				Namespace: metav1.NamespaceSystem,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
		}
	)

	return registry.AddAllAndSerialize(
		clusterRole,
		clusterRoleBinding,
		role,
		roleBinding,
	)
}

func (m *machineControllerManager) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyClusterRoleBindingRuntime() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager-" + m.namespace}}
}

func (m *machineControllerManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: m.namespace}}
}

func (m *machineControllerManager) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameMachineControllerManager, m.namespace)
}

func (m *machineControllerManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: VPAName, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceTargetName, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyManagedResourceSecret() *corev1.Secret {
	// TODO(dimityrmirchev): Remove this once mr secrets are turned into garbage-collectable, immutable secrets, after Gardener v1.90
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "managedresource-" + managedResourceTargetName, Namespace: m.namespace}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: "machine-controller-manager",
	}
}
