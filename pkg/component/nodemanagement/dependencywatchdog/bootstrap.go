// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dependencywatchdog

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// Role is a string alias type.
type Role string

const (
	// RoleWeeder is a constant for the 'weeder' role of the dependency-watchdog.
	RoleWeeder Role = "weeder"
	// RoleProber is a constant for the 'prober' role of the dependency-watchdog.
	RoleProber Role = "prober"

	prefixDependencyWatchdog       = "dependency-watchdog"
	volumeName                     = "config"
	volumeMountPath                = "/etc/dependency-watchdog/config"
	configFileName                 = "dep-config.yaml"
	dwdWeederDefaultLockObjectName = "dwd-weeder-leader-election"
	dwdProberDefaultLockObjectName = "dwd-prober-leader-election"
)

// BootstrapperValues contains dependency-watchdog values.
type BootstrapperValues struct {
	// Role defines which dependency-watchdog controller i.e. weeder or prober.
	Role Role
	// WeederConfig is the Config for the weeder Role.
	WeederConfig weederapi.Config
	// ProberConfig is the Config for the prober Role.
	ProberConfig proberapi.Config
	// Image is the container image used for DependencyWatchdog.
	Image string
	// KubernetesVersion is the Kubernetes version of the Seed.
	KubernetesVersion *semver.Version
}

// NewBootstrapper creates a new instance of DeployWaiter for the dependency-watchdog.
func NewBootstrapper(
	client client.Client,
	namespace string,
	values BootstrapperValues,
) component.DeployWaiter {
	return &bootstrapper{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type bootstrapper struct {
	client    client.Client
	namespace string
	values    BootstrapperValues
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	var (
		err      error
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	)

	configMap, err := b.getConfigMap()
	if err != nil {
		return err
	}
	serviceAccount := b.getServiceAccount()
	clusterRole := b.getClusterRole()
	clusterRoleBinding := b.getClusterRoleBinding(serviceAccount, clusterRole)
	role := b.getRole()
	roleBinding := b.getRoleBinding(serviceAccount, role)
	deployment := b.getDeployment(serviceAccount.Name, configMap.Name)
	vpa := b.getVPA(deployment.Name)
	podDisruptionBudget := b.getPDB(deployment)

	resources, err := registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		role,
		roleBinding,
		configMap,
		deployment,
		podDisruptionBudget,
		vpa,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, b.client, b.namespace, b.name(), false, resources)
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, b.client, b.namespace, b.name())
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *bootstrapper) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, b.name())
}

func (b *bootstrapper) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, b.name())
}

func (b *bootstrapper) getConfigMap() (*corev1.ConfigMap, error) {
	var (
		config string
		err    error
	)

	switch b.values.Role {
	case RoleWeeder:
		config, err = encodeConfig(&b.values.WeederConfig)
		if err != nil {
			return nil, err
		}

	case RoleProber:
		config, err = encodeConfig(&b.values.ProberConfig)
		if err != nil {
			return nil, err
		}
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name() + "-config",
			Namespace: b.namespace,
			Labels:    map[string]string{v1beta1constants.LabelApp: b.name()},
		},
		Data: map[string]string{configFileName: config},
	}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return configMap, nil
}

func (b *bootstrapper) getServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name(),
			Namespace: b.namespace,
		},
		AutomountServiceAccountToken: ptr.To(false),
	}
}

func (b *bootstrapper) getClusterRole() *rbacv1.ClusterRole {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("gardener.cloud:%s", b.name()),
		},
		Rules: b.getClusterRolePolicyRules(),
	}
	return clusterRole
}

func (b *bootstrapper) getClusterRoleBinding(serviceAccount *corev1.ServiceAccount, clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("gardener.cloud:%s", b.name()),
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
	return clusterRoleBinding
}

func (b *bootstrapper) getRole() *rbacv1.Role {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("gardener.cloud:%s", b.name()),
			Namespace: b.namespace,
		},
		Rules: b.getRolePolicyRules(),
	}
	return role
}

func (b *bootstrapper) getRoleBinding(serviceAccount *corev1.ServiceAccount, role *rbacv1.Role) *rbacv1.RoleBinding {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("gardener.cloud:%s", b.name()),
			Namespace: b.namespace,
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
	return roleBinding
}

func (b *bootstrapper) name() string {
	return fmt.Sprintf("%s-%s", prefixDependencyWatchdog, b.values.Role)
}

func (b *bootstrapper) getLabels() map[string]string {
	return map[string]string{v1beta1constants.LabelApp: b.name()}
}

func (b *bootstrapper) getRolePolicyRules() []rbacv1.PolicyRule {
	resourceName := dwdProberDefaultLockObjectName
	if b.values.Role == RoleWeeder {
		resourceName = dwdWeederDefaultLockObjectName
	}

	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups:     []string{"coordination.k8s.io"},
			ResourceNames: []string{resourceName},
			Resources:     []string{"leases"},
			Verbs:         []string{"get", "watch", "update"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "get", "update", "patch"},
		},
	}
}

func (b *bootstrapper) getClusterRolePolicyRules() []rbacv1.PolicyRule {
	switch b.values.Role {
	case RoleWeeder:
		return []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch", "delete"},
			},
		}

	case RoleProber:
		return []rbacv1.PolicyRule{
			{
				APIGroups: []string{"extensions.gardener.cloud"},
				Resources: []string{"clusters"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "deployments/scale"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
			{
				APIGroups: []string{"machine.sapcloud.io"},
				Resources: []string{"machines"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
	}

	return nil
}

func (b *bootstrapper) getPodLabels() map[string]string {
	switch b.values.Role {
	case RoleWeeder:
		return utils.MergeStringMaps(b.getLabels(), map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})

	case RoleProber:
		return utils.MergeStringMaps(b.getLabels(), map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyIstioIngressNamespaceAlias+"-"+v1beta1constants.DefaultSNIIngressServiceName, 9443):                v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	return nil
}

func (b *bootstrapper) getContainerCommand() []string {
	switch b.values.Role {
	case RoleWeeder:
		return []string{
			"/usr/local/bin/dependency-watchdog",
			"weeder",
			fmt.Sprintf("--config-file=%s/%s", volumeMountPath, configFileName),
			"--enable-leader-election=true",
		}

	case RoleProber:
		return []string{
			"/usr/local/bin/dependency-watchdog",
			"prober",
			fmt.Sprintf("--config-file=%s/%s", volumeMountPath, configFileName),
			"--kube-api-qps=20.0",
			"--kube-api-burst=100",
			"--zap-log-level=INFO",
			"--enable-leader-election=true",
		}
	}

	return nil
}

func (b *bootstrapper) getDeployment(serviceAccountName string, configMapName string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name(),
			Namespace: b.namespace,
			Labels: utils.MergeStringMaps(map[string]string{
				resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
			}, b.getLabels()),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](2),
			Selector:             &metav1.LabelSelector{MatchLabels: b.getLabels()},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: b.getPodLabels(),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:             v1beta1constants.PriorityClassNameSeedSystem800,
					ServiceAccountName:            serviceAccountName,
					TerminationGracePeriodSeconds: ptr.To[int64](5),
					Containers: []corev1.Container{{
						Name:            prefixDependencyWatchdog,
						Image:           b.values.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         b.getContainerCommand(),
						Ports: []corev1.ContainerPort{{
							Name:          "metrics",
							ContainerPort: 9643,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      volumeName,
							MountPath: volumeMountPath,
							ReadOnly:  true,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: volumeName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMapName,
								},
							},
						},
					}},
				},
			},
		},
	}

	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment
}

func (b *bootstrapper) getPDB(deployment *appsv1.Deployment) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name(),
			Namespace: deployment.Namespace,
			Labels:    b.getLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt32(1)),
			Selector:       deployment.Spec.Selector,
		},
	}

	kubernetesutils.SetAlwaysAllowEviction(pdb, b.values.KubernetesVersion)

	return pdb
}

func (b *bootstrapper) getVPA(deploymentName string) *vpaautoscalingv1.VerticalPodAutoscaler {
	var (
		vpaMinAllowedMemory = "25Mi"
		updateMode          = vpaautoscalingv1.UpdateModeAuto
	)

	if b.values.Role == RoleProber {
		vpaMinAllowedMemory = "50Mi"
	}

	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name(),
			Namespace: b.namespace,
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       deploymentName,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: &updateMode,
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
					ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse(vpaMinAllowedMemory),
					},
				}},
			},
		},
	}
}

func encodeConfig[T any](config *T) (string, error) {
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
