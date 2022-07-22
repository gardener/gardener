// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package clusterautoscaler

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ServiceName is the name of the service of the cluster-autoscaler.
	ServiceName = "cluster-autoscaler"

	managedResourceTargetName = "shoot-core-cluster-autoscaler"
	containerName             = v1beta1constants.DeploymentNameClusterAutoscaler

	portNameMetrics       = "metrics"
	portMetrics     int32 = 8085
)

// Interface contains functions for a cluster-autoscaler deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetNamespaceUID sets the UID of the namespace into which the cluster-autoscaler shall be deployed.
	SetNamespaceUID(types.UID)
	// SetMachineDeployments sets the machine deployments.
	SetMachineDeployments([]extensionsv1alpha1.MachineDeployment)
}

// New creates a new instance of DeployWaiter for the cluster-autoscaler.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	image string,
	replicas int32,
	config *gardencorev1beta1.ClusterAutoscaler,
) Interface {
	return &clusterAutoscaler{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		image:          image,
		replicas:       replicas,
		config:         config,
	}
}

type clusterAutoscaler struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	image          string
	replicas       int32
	config         *gardencorev1beta1.ClusterAutoscaler

	namespaceUID       types.UID
	machineDeployments []extensionsv1alpha1.MachineDeployment
}

func (c *clusterAutoscaler) Deploy(ctx context.Context) error {
	var (
		shootAccessSecret  = c.newShootAccessSecret()
		serviceAccount     = c.emptyServiceAccount()
		clusterRoleBinding = c.emptyClusterRoleBinding()
		vpa                = c.emptyVPA()
		service            = c.emptyService()
		deployment         = c.emptyDeployment()

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		command          = c.computeCommand()
	)

	genericTokenKubeconfigSecret, found := c.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c.client, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c.client, clusterRoleBinding, func() error {
		clusterRoleBinding.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         "v1",
			Kind:               "Namespace",
			Name:               c.namespace,
			UID:                c.namespaceUID,
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		}}
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleControlName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccount.Name,
			Namespace: c.namespace,
		}}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, service, func() error {
		service.Labels = getLabels()
		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		desiredPorts := []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     portMetrics,
			},
		}
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, c.client); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		})
		deployment.Spec.Replicas = &c.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart:         "true",
					v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToSeedAPIServer:  v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyFromPrometheus:   v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName:            serviceAccount.Name,
				TerminationGracePeriodSeconds: pointer.Int64(5),
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           c.image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         command,
						Ports: []corev1.ContainerPort{
							{
								Name:          portNameMetrics,
								ContainerPort: portMetrics,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name:  "CONTROL_NAMESPACE",
								Value: c.namespace,
							},
							{
								Name:  "TARGET_KUBECONFIG",
								Value: gutil.PathGenericKubeconfig,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("300Mi"),
							},
						},
					},
				},
			},
		}

		utilruntime.Must(gutil.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameClusterAutoscaler,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					ControlledValues: &controlledValues,
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	data, err := c.computeShootResourcesData(shootAccessSecret.ServiceAccountName)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForShoot(ctx, c.client, c.namespace, managedResourceTargetName, false, data); err != nil {
		return err
	}

	return nil
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.DeploymentNameClusterAutoscaler,
	}
}

func (c *clusterAutoscaler) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(
		ctx,
		c.client,
		c.emptyManagedResource(),
		c.emptyManagedResourceSecret(),
		c.emptyVPA(),
		c.emptyDeployment(),
		c.emptyClusterRoleBinding(),
		c.newShootAccessSecret().Secret,
		c.emptyService(),
		c.emptyServiceAccount(),
	)
}

func (c *clusterAutoscaler) Wait(_ context.Context) error        { return nil }
func (c *clusterAutoscaler) WaitCleanup(_ context.Context) error { return nil }
func (c *clusterAutoscaler) SetNamespaceUID(uid types.UID)       { c.namespaceUID = uid }
func (c *clusterAutoscaler) SetMachineDeployments(machineDeployments []extensionsv1alpha1.MachineDeployment) {
	c.machineDeployments = machineDeployments
}

func (c *clusterAutoscaler) emptyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler-" + c.namespace}}
}

func (c *clusterAutoscaler) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler", Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler-vpa", Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: c.namespace}}
}

func (c *clusterAutoscaler) newShootAccessSecret() *gutil.ShootAccessSecret {
	return gutil.NewShootAccessSecret(v1beta1constants.DeploymentNameClusterAutoscaler, c.namespace)
}

func (c *clusterAutoscaler) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameClusterAutoscaler, Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceTargetName, Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyManagedResourceSecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedresources.SecretName(managedResourceTargetName, true), Namespace: c.namespace}}
}

func (c *clusterAutoscaler) computeCommand() []string {
	var (
		command = []string{
			"./cluster-autoscaler",
			fmt.Sprintf("--address=:%d", portMetrics),
			"--kubeconfig=" + gutil.PathGenericKubeconfig,
			"--cloud-provider=mcm",
			"--stderrthreshold=info",
			"--skip-nodes-with-system-pods=false",
			"--skip-nodes-with-local-storage=false",
			"--expendable-pods-priority-cutoff=-10",
			"--balance-similar-node-groups=true",
			"--v=2",
		}
	)

	if c.config == nil {
		c.config = &gardencorev1beta1.ClusterAutoscaler{}
	}
	gardencorev1beta1.SetDefaults_ClusterAutoscaler(c.config)

	command = append(command,
		fmt.Sprintf("--expander=%s", *c.config.Expander),
		fmt.Sprintf("--max-graceful-termination-sec=%d", *c.config.MaxGracefulTerminationSeconds),
		fmt.Sprintf("--max-node-provision-time=%s", c.config.MaxNodeProvisionTime.Duration),
		fmt.Sprintf("--scale-down-utilization-threshold=%f", *c.config.ScaleDownUtilizationThreshold),
		fmt.Sprintf("--scale-down-unneeded-time=%s", c.config.ScaleDownUnneededTime.Duration),
		fmt.Sprintf("--scale-down-delay-after-add=%s", c.config.ScaleDownDelayAfterAdd.Duration),
		fmt.Sprintf("--scale-down-delay-after-delete=%s", c.config.ScaleDownDelayAfterDelete.Duration),
		fmt.Sprintf("--scale-down-delay-after-failure=%s", c.config.ScaleDownDelayAfterFailure.Duration),
		fmt.Sprintf("--scan-interval=%s", c.config.ScanInterval.Duration),
	)

	for _, taint := range c.config.IgnoreTaints {
		command = append(command, fmt.Sprintf("--ignore-taint=%s", taint))
	}

	for _, machineDeployment := range c.machineDeployments {
		command = append(command, fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment.Minimum, machineDeployment.Maximum, c.namespace, machineDeployment.Name))
	}

	return command
}

func (c *clusterAutoscaler) computeShootResourcesData(serviceAccountName string) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:cluster-autoscaler",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"events", "endpoints"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/eviction"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/status"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"endpoints"},
					ResourceNames: []string{ServiceName},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"watch", "list", "get", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces", "pods", "services", "replicationcontrollers", "persistentvolumeclaims", "persistentvolumes"},
					Verbs:     []string{"watch", "list", "get"},
				},
				{
					APIGroups: []string{"apps", "extensions"},
					Resources: []string{"daemonsets", "replicasets", "statefulsets"},
					Verbs:     []string{"watch", "list", "get"},
				},
				{
					APIGroups: []string{"policy"},
					Resources: []string{"poddisruptionbudgets"},
					Verbs:     []string{"watch", "list"},
				},
				{
					APIGroups: []string{"storage.k8s.io"},
					Resources: []string{"storageclasses", "csinodes", "csidrivers", "csistoragecapacities"},
					Verbs:     []string{"watch", "list", "get"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"cluster-autoscaler"},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups: []string{"batch", "extensions"},
					Resources: []string{"jobs"},
					Verbs:     []string{"get", "list", "patch", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs", "cronjobs"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:cluster-autoscaler",
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
				Name:      "gardener.cloud:target:cluster-autoscaler",
				Namespace: metav1.NamespaceSystem,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"watch", "list", "get", "create"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"cluster-autoscaler-status"},
					Verbs:         []string{"delete", "update"},
				},
			},
		}

		rolebinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:cluster-autoscaler",
				Namespace: metav1.NamespaceSystem,
			},
			Subjects: []rbacv1.Subject{{
				Kind: rbacv1.ServiceAccountKind,
				Name: serviceAccountName,
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
		rolebinding,
	)
}
