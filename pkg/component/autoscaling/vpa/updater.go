// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	updater                  = "vpa-updater"
	updaterPortServer  int32 = 8080
	updaterPortMetrics int32 = 8943
)

// ValuesUpdater is a set of configuration values for the vpa-updater.
type ValuesUpdater struct {
	// EvictAfterOOMThreshold defines the threshold that will lead to pod eviction in case it OOMed in less than the given
	// threshold since its start and if it has only one container.
	EvictAfterOOMThreshold *metav1.Duration
	// EvictionRateBurst defines the burst of pods that can be evicted.
	EvictionRateBurst *int32
	// EvictionRateLimit defines the number of pods that can be evicted per second. A rate limit set to 0 or -1 will
	// disable the rate limiter.
	EvictionRateLimit *float64
	// EvictionTolerance defines the fraction of replica count that can be evicted for update in case more than one
	// pod can be evicted.
	EvictionTolerance *float64
	// Image is the container image.
	Image string
	// Interval is the interval how often the updater should run.
	Interval *metav1.Duration
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas is the number of pod replicas.
	Replicas *int32
}

func (v *vpa) updaterResourceConfigs() component.ResourceConfigs {
	var (
		clusterRole              = v.emptyClusterRole("evictioner")
		clusterRoleBinding       = v.emptyClusterRoleBinding("evictioner")
		roleLeaderLocking        = v.emptyRole("leader-locking-vpa-updater")
		roleBindingLeaderLocking = v.emptyRoleBinding("leader-locking-vpa-updater")
		deployment               = v.emptyDeployment(updater)
		podDisruptionBudget      = v.emptyPodDisruptionBudget(updater)
		vpa                      = v.emptyVerticalPodAutoscaler(updater)
	)

	configs := component.ResourceConfigs{
		{Obj: clusterRole, Class: component.Application, MutateFn: func() { v.reconcileUpdaterClusterRole(clusterRole) }},
		{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() { v.reconcileUpdaterClusterRoleBinding(clusterRoleBinding, clusterRole, updater) }},
		{Obj: roleLeaderLocking, Class: component.Application, MutateFn: func() { v.reconcileUpdaterRoleLeaderLocking(roleLeaderLocking) }},
		{Obj: roleBindingLeaderLocking, Class: component.Application, MutateFn: func() {
			v.reconcileUpdaterRoleBindingLeaderLocking(roleBindingLeaderLocking, roleLeaderLocking, updater)
		}},
		{Obj: vpa, Class: component.Runtime, MutateFn: func() { v.reconcileUpdaterVPA(vpa, deployment) }},
	}

	if v.values.ClusterType == component.ClusterTypeSeed {
		serviceAccount := v.emptyServiceAccount(updater)
		configs = append(configs,
			component.ResourceConfig{Obj: serviceAccount, Class: component.Application, MutateFn: func() { v.reconcileUpdaterServiceAccount(serviceAccount) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileUpdaterDeployment(deployment, &serviceAccount.Name) }},
			component.ResourceConfig{Obj: podDisruptionBudget, Class: component.Runtime, MutateFn: func() { v.reconcilePodDisruptionBudget(podDisruptionBudget, deployment) }},
		)
	} else {
		configs = append(configs,
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileUpdaterDeployment(deployment, nil) }},
			component.ResourceConfig{Obj: podDisruptionBudget, Class: component.Runtime, MutateFn: func() { v.reconcilePodDisruptionBudget(podDisruptionBudget, deployment) }},
		)
	}

	return configs
}

func (v *vpa) reconcileUpdaterServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = getRoleLabel()
	serviceAccount.AutomountServiceAccountToken = ptr.To(false)
}

func (v *vpa) reconcileUpdaterClusterRole(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{"replicasets"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/eviction"},
			Verbs:     []string{"create"},
		},
	}
}

func (v *vpa) reconcileUpdaterClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole, serviceAccountName string) {
	clusterRoleBinding.Labels = getRoleLabel()
	clusterRoleBinding.Annotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      serviceAccountName,
		Namespace: v.namespaceForApplicationClassResource(),
	}}
}

func (v *vpa) reconcileUpdaterRoleLeaderLocking(role *rbacv1.Role) {
	role.Labels = getRoleLabel()
	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups:     []string{"coordination.k8s.io"},
			Resources:     []string{"leases"},
			ResourceNames: []string{updater},
			Verbs:         []string{"get", "watch", "update"},
		},
	}
}

func (v *vpa) reconcileUpdaterRoleBindingLeaderLocking(roleBinding *rbacv1.RoleBinding, role *rbacv1.Role, serviceAccountName string) {
	roleBinding.Labels = getRoleLabel()
	roleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}
	roleBinding.Subjects = []rbacv1.Subject{{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      serviceAccountName,
		Namespace: v.namespaceForApplicationClassResource(),
	}}
}

func (v *vpa) reconcileUpdaterDeployment(deployment *appsv1.Deployment, serviceAccountName *string) {
	deployment.Labels = utils.MergeStringMaps(v.getDeploymentLabels(updater), map[string]string{
		resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
	})
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             ptr.To(ptr.Deref(v.values.Updater.Replicas, 1)),
		RevisionHistoryLimit: ptr.To[int32](2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(updater)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getAllLabels(updater), map[string]string{
					v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				PriorityClassName: v.values.Updater.PriorityClassName,
				Containers: []corev1.Container{{
					Name:            "updater",
					Image:           v.values.Updater.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         v.computeUpdaterCommands(),
					Args: []string{
						"--min-replicas=1",
						fmt.Sprintf("--eviction-tolerance=%f", ptr.Deref(v.values.Updater.EvictionTolerance, gardencorev1beta1.DefaultEvictionTolerance)),
						fmt.Sprintf("--eviction-rate-burst=%d", ptr.Deref(v.values.Updater.EvictionRateBurst, gardencorev1beta1.DefaultEvictionRateBurst)),
						fmt.Sprintf("--eviction-rate-limit=%f", ptr.Deref(v.values.Updater.EvictionRateLimit, gardencorev1beta1.DefaultEvictionRateLimit)),
						fmt.Sprintf("--evict-after-oom-threshold=%s", ptr.Deref(v.values.Updater.EvictAfterOOMThreshold, gardencorev1beta1.DefaultEvictAfterOOMThreshold).Duration),
						fmt.Sprintf("--updater-interval=%s", ptr.Deref(v.values.Updater.Interval, gardencorev1beta1.DefaultUpdaterInterval).Duration),
						"--stderrthreshold=info",
						"--v=2",
						"--kube-api-qps=100",
						"--kube-api-burst=120",
						"--leader-elect=true",
						fmt.Sprintf("--leader-elect-resource-namespace=%s", v.namespaceForApplicationClassResource()),
					},
					LivenessProbe: newDefaultLivenessProbe(),
					Ports: []corev1.ContainerPort{
						{
							Name:          serverPortName,
							ContainerPort: updaterPortServer,
						},
						{
							Name:          metricsPortName,
							ContainerPort: updaterPortMetrics,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("15Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
					},
				}},
			},
		},
	}

	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})

	case component.ClusterTypeShoot:
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	v.injectAPIServerConnectionSpec(deployment, updater, serviceAccountName)
}

func (v *vpa) reconcileUpdaterVPA(vpa *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) {
	vpa.Spec = vpaautoscalingv1.VerticalPodAutoscalerSpec{
		TargetRef: &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       deployment.Name,
		},
		UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto)},
		ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName:    "*",
					ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
				},
			},
		},
	}
}

func (v *vpa) computeUpdaterCommands() []string {
	out := []string{"./updater"}

	if v.values.ClusterType == component.ClusterTypeShoot {
		out = append(out, "--kubeconfig="+gardenerutils.PathGenericKubeconfig)
	}
	return out
}
