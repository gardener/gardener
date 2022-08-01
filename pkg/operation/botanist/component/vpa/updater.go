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

package vpa

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
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
	// Replicas is the number of pod replicas.
	Replicas int32
}

func (v *vpa) updaterResourceConfigs() component.ResourceConfigs {
	var (
		clusterRole        = v.emptyClusterRole("evictioner")
		clusterRoleBinding = v.emptyClusterRoleBinding("evictioner")
		deployment         = v.emptyDeployment(updater)
		vpa                = v.emptyVerticalPodAutoscaler(updater)
	)

	configs := component.ResourceConfigs{
		{Obj: clusterRole, Class: component.Application, MutateFn: func() { v.reconcileUpdaterClusterRole(clusterRole) }},
		{Obj: clusterRoleBinding, Class: component.Application, MutateFn: func() { v.reconcileUpdaterClusterRoleBinding(clusterRoleBinding, clusterRole, updater) }},
		{Obj: vpa, Class: component.Runtime, MutateFn: func() { v.reconcileUpdaterVPA(vpa, deployment) }},
	}

	if v.values.ClusterType == component.ClusterTypeSeed {
		serviceAccount := v.emptyServiceAccount(updater)
		configs = append(configs,
			component.ResourceConfig{Obj: serviceAccount, Class: component.Application, MutateFn: func() { v.reconcileUpdaterServiceAccount(serviceAccount) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileUpdaterDeployment(deployment, &serviceAccount.Name) }},
		)
	} else {
		configs = append(configs,
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileUpdaterDeployment(deployment, nil) }},
		)
	}

	return configs
}

func (v *vpa) reconcileUpdaterServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = getRoleLabel()
	serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
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
		Namespace: v.serviceAccountNamespace(),
	}}
}

func (v *vpa) reconcileUpdaterDeployment(deployment *appsv1.Deployment, serviceAccountName *string) {
	priorityClassName := v1beta1constants.PriorityClassNameSeedSystem700
	if v.values.ClusterType == component.ClusterTypeShoot {
		priorityClassName = v1beta1constants.PriorityClassNameShootControlPlane200
	}

	deployment.Labels = v.getDeploymentLabels(updater)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             &v.values.Updater.Replicas,
		RevisionHistoryLimit: pointer.Int32(2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(updater)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getAllLabels(updater), map[string]string{
					v1beta1constants.LabelNetworkPolicyFromPrometheus: v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				PriorityClassName: priorityClassName,
				Containers: []corev1.Container{{
					Name:            "updater",
					Image:           v.values.Updater.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         v.computeUpdaterCommands(),
					Args: []string{
						"--min-replicas=1",
						fmt.Sprintf("--eviction-tolerance=%f", pointer.Float64Deref(v.values.Updater.EvictionTolerance, gardencorev1beta1.DefaultEvictionTolerance)),
						fmt.Sprintf("--eviction-rate-burst=%d", pointer.Int32Deref(v.values.Updater.EvictionRateBurst, gardencorev1beta1.DefaultEvictionRateBurst)),
						fmt.Sprintf("--eviction-rate-limit=%f", pointer.Float64Deref(v.values.Updater.EvictionRateLimit, gardencorev1beta1.DefaultEvictionRateLimit)),
						fmt.Sprintf("--evict-after-oom-threshold=%s", durationDeref(v.values.Updater.EvictAfterOOMThreshold, gardencorev1beta1.DefaultEvictAfterOOMThreshold).Duration),
						fmt.Sprintf("--updater-interval=%s", durationDeref(v.values.Updater.Interval, gardencorev1beta1.DefaultUpdaterInterval).Duration),
						"--stderrthreshold=info",
						"--v=2",
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "server",
							ContainerPort: updaterPortServer,
						},
						{
							Name:          "metrics",
							ContainerPort: updaterPortMetrics,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("30m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				}},
			},
		},
	}

	if v.values.ClusterType == component.ClusterTypeShoot {
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	v.injectAPIServerConnectionSpec(deployment, updater, serviceAccountName)
}

func (v *vpa) reconcileUpdaterVPA(vpa *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) {
	updateMode := vpaautoscalingv1.UpdateModeAuto
	controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly

	vpa.Spec = vpaautoscalingv1.VerticalPodAutoscalerSpec{
		TargetRef: &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       deployment.Name,
		},
		UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &updateMode},
		ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName:    "*",
					ControlledValues: &controlledValues,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
			},
		},
	}
}

func (v *vpa) computeUpdaterCommands() []string {
	out := []string{"./updater"}

	if v.values.ClusterType == component.ClusterTypeShoot {
		out = append(out, "--kubeconfig="+gutil.PathGenericKubeconfig)
	}
	return out
}
