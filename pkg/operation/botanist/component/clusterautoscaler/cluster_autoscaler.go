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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// ServiceName is the name of the service of the cluster-autoscaler.
	ServiceName = "cluster-autoscaler"
	// SecretName is a constant for the secret name for the cluster-autoscaler's kubeconfig secret.
	SecretName = "cluster-autoscaler"
	// UserName is the name that should be used for the secret that the cluster-autoscaler uses to
	// authenticate itself with the kube-apiserver (e.g., the common name in its client certificate).
	UserName = "system:cluster-autoscaler"

	managedResourceTargetName = "shoot-core-cluster-autoscaler"
	containerName             = v1beta1constants.DeploymentNameClusterAutoscaler

	portNameMetrics                 = "metrics"
	portMetrics               int32 = 8085
	volumeMountPathKubeconfig       = "/var/lib/cluster-autoscaler"
)

// Interface contains functions for a cluster-autoscaler deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
	// SetNamespaceUID sets the UID of the namespace into which the cluster-autoscaler shall be deployed.
	SetNamespaceUID(types.UID)
	// SetMachineDeployments sets the machine deployments.
	SetMachineDeployments([]extensionsv1alpha1.MachineDeployment)
}

// New creates a new instance of DeployWaiter for the cluster-autoscaler.
func New(
	client client.Client,
	namespace string,
	image string,
	replicas int32,
	config *gardencorev1beta1.ClusterAutoscaler,
) Interface {
	return &clusterAutoscaler{
		client:    client,
		namespace: namespace,
		image:     image,
		replicas:  replicas,
		config:    config,
	}
}

type clusterAutoscaler struct {
	client    client.Client
	namespace string
	image     string
	replicas  int32
	config    *gardencorev1beta1.ClusterAutoscaler

	secrets            Secrets
	namespaceUID       types.UID
	machineDeployments []extensionsv1alpha1.MachineDeployment
}

func (c *clusterAutoscaler) Deploy(ctx context.Context) error {
	if c.secrets.Kubeconfig.Name == "" || c.secrets.Kubeconfig.Checksum == "" {
		return fmt.Errorf("missing kubeconfig secret information")
	}

	var (
		serviceAccount     = c.emptyServiceAccount()
		clusterRoleBinding = c.emptyClusterRoleBinding()
		vpa                = c.emptyVPA()
		service            = c.emptyService()
		deployment         = c.emptyDeployment()

		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto
		command       = c.computeCommand()
	)

	if err := c.client.Create(ctx, serviceAccount); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c.client, clusterRoleBinding, func() error {
		clusterRoleBinding.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         "v1",
			Kind:               "Namespace",
			Name:               c.namespace,
			UID:                c.namespaceUID,
			Controller:         pointer.BoolPtr(true),
			BlockOwnerDeletion: pointer.BoolPtr(true),
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

	if _, err := controllerutil.CreateOrUpdate(ctx, c.client, service, func() error {
		service.Labels = getLabels()
		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     portMetrics,
			},
		})
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		})
		deployment.Spec.Replicas = &c.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32Ptr(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"checksum/secret-" + c.secrets.Kubeconfig.Name: c.secrets.Kubeconfig.Checksum,
				},
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.DeprecatedGardenRole:               v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToSeedAPIServer:  v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyFromPrometheus:   v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName:            serviceAccount.Name,
				TerminationGracePeriodSeconds: pointer.Int64Ptr(5),
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
								Value: volumeMountPathKubeconfig + "/" + secrets.DataKeyKubeconfig,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("300Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("3000Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      c.secrets.Kubeconfig.Name,
								MountPath: volumeMountPathKubeconfig,
								ReadOnly:  true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: c.secrets.Kubeconfig.Name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: c.secrets.Kubeconfig.Name,
							},
						},
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameClusterAutoscaler,
		}
		vpa.Spec.UpdatePolicy = &autoscalingv1beta2.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &autoscalingv1beta2.PodResourcePolicy{
			ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
				{
					ContainerName: autoscalingv1beta2.DefaultContainerResourcePolicy,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	data, err := c.computeShootResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, c.client, c.namespace, managedResourceTargetName, false, data)
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
		c.emptyService(),
		c.emptyServiceAccount(),
	)
}

func (c *clusterAutoscaler) Wait(_ context.Context) error        { return nil }
func (c *clusterAutoscaler) WaitCleanup(_ context.Context) error { return nil }
func (c *clusterAutoscaler) SetSecrets(secrets Secrets)          { c.secrets = secrets }
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

func (c *clusterAutoscaler) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler-vpa", Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: c.namespace}}
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
		scaleDownUnneededTime  = metav1.Duration{Duration: 30 * time.Minute}
		scaleDownDelayAfterAdd = metav1.Duration{Duration: time.Hour}

		command = []string{
			"./cluster-autoscaler",
			fmt.Sprintf("--address=:%d", portMetrics),
			fmt.Sprintf("--kubeconfig=%s", volumeMountPathKubeconfig+"/"+secrets.DataKeyKubeconfig),
			"--cloud-provider=mcm",
			"--stderrthreshold=info",
			"--skip-nodes-with-system-pods=false",
			"--skip-nodes-with-local-storage=false",
			"--expander=least-waste",
			"--expendable-pods-priority-cutoff=-10",
			"--balance-similar-node-groups=true",
			"--v=2",
		}
	)

	if c.config != nil {
		if val := c.config.ScaleDownUtilizationThreshold; val != nil {
			command = append(command, fmt.Sprintf("--scale-down-utilization-threshold=%f", *val))
		}
		if val := c.config.ScaleDownUnneededTime; val != nil {
			scaleDownUnneededTime = *val
		}
		if val := c.config.ScaleDownDelayAfterAdd; val != nil {
			scaleDownDelayAfterAdd = *val
		}
		if val := c.config.ScaleDownDelayAfterFailure; val != nil {
			command = append(command, fmt.Sprintf("--scale-down-delay-after-failure=%s", val.Duration))
		}
		if val := c.config.ScaleDownDelayAfterDelete; val != nil {
			command = append(command, fmt.Sprintf("--scale-down-delay-after-delete=%s", val.Duration))
		}
		if val := c.config.ScanInterval; val != nil {
			command = append(command, fmt.Sprintf("--scan-interval=%s", val.Duration))
		}
	}

	command = append(command,
		fmt.Sprintf("--scale-down-unneeded-time=%s", scaleDownUnneededTime.Duration),
		fmt.Sprintf("--scale-down-delay-after-add=%s", scaleDownDelayAfterAdd.Duration),
	)

	for _, machineDeployment := range c.machineDeployments {
		command = append(command, fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment.Minimum, machineDeployment.Maximum, c.namespace, machineDeployment.Name))
	}

	return command
}

func (c *clusterAutoscaler) computeShootResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:cluster-autoscaler-shoot",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"events", "endpoints"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/eviction", "configmaps"},
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
					Resources: []string{"pods", "services", "replicationcontrollers", "persistentvolumeclaims", "persistentvolumes"},
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
					Resources: []string{"storageclasses", "csinodes"},
					Verbs:     []string{"watch", "list", "get"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"cluster-autoscaler-status"},
					Verbs:         []string{"delete", "get", "update"},
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
				Name: "system:cluster-autoscaler-shoot",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: UserName,
			}},
		}
	)

	return registry.AddAllAndSerialize(
		clusterRole,
		clusterRoleBinding,
	)
}

// Secrets is collection of secrets for the cluster-autoscaler.
type Secrets struct {
	// Kubeconfig is a secret which can be used by the cluster-autoscaler to communicate to the kube-apiserver.
	Kubeconfig component.Secret
}
