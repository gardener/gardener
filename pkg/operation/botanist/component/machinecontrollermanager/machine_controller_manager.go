// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	portMetrics = 10258
)

// New creates a new instance of DeployWaiter for the machine-controller-manager.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &machineControllerManager{
		client:                        client,
		namespace:                     namespace,
		secretsManager:                secretsManager,
		values:                        values,
		runtimeVersionGreaterEqual123: versionutils.ConstraintK8sGreaterEqual123.Check(values.RuntimeKubernetesVersion),
	}
}

type machineControllerManager struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values

	runtimeVersionGreaterEqual123 bool
}

// Values is a set of configuration values for the machine-controller-manager component.
type Values struct {
	// Image is the container image used for machine-controller-manager.
	Image string
	// NamespaceUID is the UID of the namespace.
	NamespaceUID types.UID
	// Replicas is the number of replicas for the deployment.
	Replicas int32
	// RuntimeKubernetesVersion is the Kubernetes version of the runtime cluster.
	RuntimeKubernetesVersion *semver.Version
}

func (m *machineControllerManager) Deploy(ctx context.Context) error {
	var (
		shootAccessSecret   = m.newShootAccessSecret()
		serviceAccount      = m.emptyServiceAccount()
		clusterRoleBinding  = m.emptyClusterRoleBindingRuntime()
		service             = m.emptyService()
		deployment          = m.emptyDeployment()
		podDisruptionBudget = m.emptyPodDisruptionBudget()
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
			UID:                m.values.NamespaceUID,
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
			Port:     utils.IntStrPtrFromInt(portMetrics),
			Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
		}))

		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		desiredPorts := []corev1.ServicePort{{
			Name:     "metrics",
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
					Name:            "machine-controller-manager",
					Image:           m.values.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"./machine-controller-manager",
						"--control-kubeconfig=inClusterConfig",
						"--delete-migrated-machine-class=true",
						"--machine-safety-apiserver-statuscheck-timeout=30s",
						"--machine-safety-apiserver-statuscheck-period=1m",
						"--machine-safety-orphan-vms-period=30m",
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
								Port:   intstr.FromInt(portMetrics),
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
						Name:          "metrics",
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
		switch pdb := podDisruptionBudget.(type) {
		case *policyv1.PodDisruptionBudget:
			pdb.Labels = utils.MergeStringMaps(pdb.Labels, getLabels())
			pdb.Spec = policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt(1),
				Selector:       deployment.Spec.Selector,
			}
		case *policyv1beta1.PodDisruptionBudget:
			pdb.Labels = utils.MergeStringMaps(pdb.Labels, getLabels())
			pdb.Spec = policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt(1),
				Selector:       deployment.Spec.Selector,
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (m *machineControllerManager) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, m.client,
		m.emptyPodDisruptionBudget(),
		m.emptyDeployment(),
		m.newShootAccessSecret().Secret,
		m.emptyService(),
		m.emptyClusterRoleBindingRuntime(),
		m.emptyServiceAccount(),
	)
}

func (m *machineControllerManager) Wait(_ context.Context) error        { return nil }
func (m *machineControllerManager) WaitCleanup(_ context.Context) error { return nil }

func (m *machineControllerManager) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyClusterRoleBindingRuntime() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager-" + m.namespace}}
}

func (m *machineControllerManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: m.namespace}}
}

func (m *machineControllerManager) newShootAccessSecret() *gardenerutils.ShootAccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameMachineControllerManager, m.namespace)
}

func (m *machineControllerManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyPodDisruptionBudget() client.Object {
	objectMeta := metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: m.namespace}

	if m.runtimeVersionGreaterEqual123 {
		return &policyv1.PodDisruptionBudget{ObjectMeta: objectMeta}
	}
	return &policyv1beta1.PodDisruptionBudget{ObjectMeta: objectMeta}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: "machine-controller-manager",
	}
}
