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
	recommender                  = "vpa-recommender"
	recommenderPortServer  int32 = 8080
	recommenderPortMetrics int32 = 8942
)

// ValuesRecommender is a set of configuration values for the vpa-recommender.
type ValuesRecommender struct {
	// RecommendationMarginFraction is the fraction of usage added as the safety margin to the recommended request.
	RecommendationMarginFraction *float64
	// Image is the container image.
	Image string
	// Interval is the interval how often the recommender should run.
	Interval *metav1.Duration
	// Replicas is the number of pod replicas.
	Replicas int32
}

func (v *vpa) recommenderResourceConfigs() component.ResourceConfigs {
	var (
		clusterRoleMetricsReader          = v.emptyClusterRole("metrics-reader")
		clusterRoleBindingMetricsReader   = v.emptyClusterRoleBinding("metrics-reader")
		clusterRoleCheckpointActor        = v.emptyClusterRole("checkpoint-actor")
		clusterRoleBindingCheckpointActor = v.emptyClusterRoleBinding("checkpoint-actor")
		deployment                        = v.emptyDeployment(recommender)
		vpa                               = v.emptyVerticalPodAutoscaler(recommender)
	)

	configs := component.ResourceConfigs{
		{Obj: clusterRoleMetricsReader, Class: component.Application, MutateFn: func() { v.reconcileRecommenderClusterRoleMetricsReader(clusterRoleMetricsReader) }},
		{Obj: clusterRoleBindingMetricsReader, Class: component.Application, MutateFn: func() {
			v.reconcileRecommenderClusterRoleBinding(clusterRoleBindingMetricsReader, clusterRoleMetricsReader, recommender)
		}},
		{Obj: clusterRoleCheckpointActor, Class: component.Application, MutateFn: func() { v.reconcileRecommenderClusterRoleCheckpointActor(clusterRoleCheckpointActor) }},
		{Obj: clusterRoleBindingCheckpointActor, Class: component.Application, MutateFn: func() {
			v.reconcileRecommenderClusterRoleBinding(clusterRoleBindingCheckpointActor, clusterRoleCheckpointActor, recommender)
		}},
		{Obj: vpa, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderVPA(vpa, deployment) }},
	}

	if v.values.ClusterType == component.ClusterTypeSeed {
		serviceAccount := v.emptyServiceAccount(recommender)
		configs = append(configs,
			component.ResourceConfig{Obj: serviceAccount, Class: component.Application, MutateFn: func() { v.reconcileRecommenderServiceAccount(serviceAccount) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderDeployment(deployment, &serviceAccount.Name) }},
		)
	} else {
		configs = append(configs,
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderDeployment(deployment, nil) }},
		)
	}

	return configs
}

func (v *vpa) reconcileRecommenderServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = getRoleLabel()
	serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
}

func (v *vpa) reconcileRecommenderClusterRoleMetricsReader(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"metrics.k8s.io"},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list"},
		},
	}
}

func (v *vpa) reconcileRecommenderClusterRoleCheckpointActor(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"poc.autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalercheckpoints"},
			Verbs:     []string{"get", "list", "watch", "create", "patch", "delete"},
		},
		{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalercheckpoints"},
			Verbs:     []string{"get", "list", "watch", "create", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "list"},
		},
	}
}

func (v *vpa) reconcileRecommenderClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole, serviceAccountName string) {
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

func (v *vpa) reconcileRecommenderDeployment(deployment *appsv1.Deployment, serviceAccountName *string) {
	priorityClassName := v1beta1constants.PriorityClassNameSeedSystem700
	if v.values.ClusterType == component.ClusterTypeShoot {
		priorityClassName = v1beta1constants.PriorityClassNameShootControlPlane200
	}

	deployment.Labels = v.getDeploymentLabels(recommender)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             &v.values.Recommender.Replicas,
		RevisionHistoryLimit: pointer.Int32(2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(recommender)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getAllLabels(recommender), map[string]string{
					v1beta1constants.LabelNetworkPolicyFromPrometheus: v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				PriorityClassName: priorityClassName,
				Containers: []corev1.Container{{
					Name:            "recommender",
					Image:           v.values.Recommender.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         v.computeRecommenderCommands(),
					Args: []string{
						"--v=3",
						"--stderrthreshold=info",
						"--pod-recommendation-min-cpu-millicores=5",
						"--pod-recommendation-min-memory-mb=10",
						fmt.Sprintf("--recommendation-margin-fraction=%f", pointer.Float64Deref(v.values.Recommender.RecommendationMarginFraction, gardencorev1beta1.DefaultRecommendationMarginFraction)),
						fmt.Sprintf("--recommender-interval=%s", durationDeref(v.values.Recommender.Interval, gardencorev1beta1.DefaultRecommenderInterval).Duration),
						"--kube-api-qps=100",
						"--kube-api-burst=120",
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "server",
							ContainerPort: recommenderPortServer,
						},
						{
							Name:          "metrics",
							ContainerPort: recommenderPortMetrics,
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

	v.injectAPIServerConnectionSpec(deployment, recommender, serviceAccountName)
}

func (v *vpa) reconcileRecommenderVPA(vpa *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) {
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
						corev1.ResourceMemory: resource.MustParse("40Mi"),
					},
				},
			},
		},
	}
}

func (v *vpa) computeRecommenderCommands() []string {
	out := []string{"./recommender"}

	if v.values.ClusterType == component.ClusterTypeShoot {
		out = append(out, "--kubeconfig="+gutil.PathGenericKubeconfig)
	}
	return out
}
