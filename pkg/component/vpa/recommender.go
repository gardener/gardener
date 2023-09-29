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

package vpa

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	recommender            = "vpa-recommender"
	recommenderPortServer  = 8080
	recommenderPortMetrics = 8942
)

// ValuesRecommender is a set of configuration values for the vpa-recommender.
type ValuesRecommender struct {
	// RecommendationMarginFraction is the fraction of usage added as the safety margin to the recommended request.
	RecommendationMarginFraction *float64
	// Image is the container image.
	Image string
	// Interval is the interval how often the recommender should run.
	Interval *metav1.Duration
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas is the number of pod replicas.
	Replicas *int32
}

func (v *vpa) recommenderResourceConfigs() component.ResourceConfigs {
	var (
		clusterRoleMetricsReader          = v.emptyClusterRole("metrics-reader")
		clusterRoleBindingMetricsReader   = v.emptyClusterRoleBinding("metrics-reader")
		clusterRoleCheckpointActor        = v.emptyClusterRole("checkpoint-actor")
		clusterRoleBindingCheckpointActor = v.emptyClusterRoleBinding("checkpoint-actor")
		service                           = v.emptyService(recommender)
		deployment                        = v.emptyDeployment(recommender)
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
		{Obj: service, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderService(service) }},
	}

	if v.values.ClusterType == component.ClusterTypeSeed {
		// We do not deploy a vpa resource for a seed recommender, since that would cause the recommender to act on
		// said vpa resource, and attempt to autoscale its own deployment. Self-scaling is not supported by VPA.
		// This difference in behavior stems from the fact that a shoot VPA is controlling another k8s cluster,
		// while a seed VPA is controlling the very same k8s cluster which is hosting it.

		serviceAccount := v.emptyServiceAccount(recommender)
		configs = append(configs,
			component.ResourceConfig{Obj: serviceAccount, Class: component.Application, MutateFn: func() { v.reconcileRecommenderServiceAccount(serviceAccount) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderDeployment(deployment, &serviceAccount.Name) }},
		)
	} else {
		vpa := v.emptyVerticalPodAutoscaler(recommender)
		configs = append(configs,
			component.ResourceConfig{Obj: vpa, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderVPA(vpa, deployment) }},
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
	var cpuRequest string
	var memoryRequest string
	if v.values.ClusterType == component.ClusterTypeShoot {
		// The recommender in the shoot control plane is subject to autoscaling. Use small values and rely on the
		// autoscaler to adjust them.
		cpuRequest = "30m"
		memoryRequest = "200Mi"
	} else {
		// Seed recommenders are not subject to autoscaling. Use values which would suffice for large seeds and soil clusters
		cpuRequest = "200m"
		memoryRequest = "800M"
	}

	// vpa-recommender is not using leader election, hence it is not capable of running multiple replicas (and as a
	// consequence, don't need a PDB).
	deployment.Labels = v.getDeploymentLabels(recommender)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             pointer.Int32(pointer.Int32Deref(v.values.Recommender.Replicas, 1)),
		RevisionHistoryLimit: pointer.Int32(2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(recommender)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getAllLabels(recommender), map[string]string{
					v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				PriorityClassName: v.values.Recommender.PriorityClassName,
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
						"--memory-saver=true",
					},
					LivenessProbe: newDefaultLivenessProbe(),
					Ports: []corev1.ContainerPort{
						{
							Name:          "server",
							ContainerPort: recommenderPortServer,
						},
						{
							Name:          metricsPortName,
							ContainerPort: recommenderPortMetrics,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cpuRequest),
							corev1.ResourceMemory: resource.MustParse(memoryRequest),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
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
		out = append(out, "--kubeconfig="+gardenerutils.PathGenericKubeconfig)
	}
	return out
}

func (v *vpa) reconcileRecommenderService(service *corev1.Service) {
	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, networkingv1.NetworkPolicyPort{
			Port:     utils.IntStrPtrFromInt32(recommenderPortMetrics),
			Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
		}))

	// TODO: For whatever reasons, the seed-prometheus also scrapes vpa-recommenders in all shoot namespaces.
	//  Conceptually, this is wrong and should be improved (seed-prometheus should only scrape vpa-recommenders in
	//  garden namespace, and prometheis in shoot namespaces should scrape their vpa-recommenders, respectively).
	case component.ClusterTypeShoot:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(service, metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}}))
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
	}

	service.Spec.Selector = getAppLabel(recommender)
	desiredPorts := []corev1.ServicePort{{
		Port:       recommenderPortMetrics,
		TargetPort: intstr.FromInt32(recommenderPortMetrics),
	}}
	service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, "")
}
