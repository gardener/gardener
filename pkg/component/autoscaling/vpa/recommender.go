// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
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
	// TargetCPUPercentile is the CPU usage percentile that will be used as a base for CPU target recommendation.
	// Doesn't affect CPU lower bound, CPU upper bound nor memory recommendations.
	TargetCPUPercentile *float64
	// RecommendationLowerBoundCPUPercentile is the usage percentile that will be used for the lower bound on CPU recommendation.
	RecommendationLowerBoundCPUPercentile *float64
	// RecommendationUpperBoundCPUPercentile is the usage percentile that will be used for the upper bound on CPU recommendation.
	RecommendationUpperBoundCPUPercentile *float64
	// CPUHistogramDecayHalfLife is the amount of time it takes a historical CPU usage sample to lose half of its weight.
	CPUHistogramDecayHalfLife *metav1.Duration
	// TargetMemoryPercentile is the usage percentile that will be used as a base for memory target recommendation.
	// Doesn't affect memory lower bound nor memory upper bound.
	TargetMemoryPercentile *float64
	// RecommendationLowerBoundMemoryPercentile is the usage percentile that will be used for the lower bound on memory recommendation.
	RecommendationLowerBoundMemoryPercentile *float64
	// RecommendationUpperBoundMemoryPercentile is the usage percentile that will be used for the upper bound on memory recommendation.
	RecommendationUpperBoundMemoryPercentile *float64
	// MemoryHistogramDecayHalfLife is the amount of time it takes a historical memory usage sample to lose half of its weight.
	MemoryHistogramDecayHalfLife *metav1.Duration
	// MemoryAggregationInterval is the length of a single interval, for which the peak memory usage is computed.
	MemoryAggregationInterval *metav1.Duration
	// MemoryAggregationIntervalCount is the number of consecutive memory-aggregation-intervals which make up the
	// MemoryAggregationWindowLength which in turn is the period for memory usage aggregation by VPA. In other words,
	// `MemoryAggregationWindowLength = memory-aggregation-interval * memory-aggregation-interval-count`.
	MemoryAggregationIntervalCount *int64
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
		clusterRoleStatusActor            = v.emptyClusterRole("status-actor")
		clusterRoleBindingStatusActor     = v.emptyClusterRoleBinding("status-actor")
		roleLeaderLocking                 = v.emptyRole("leader-locking-vpa-recommender")
		roleBindingLeaderLocking          = v.emptyRoleBinding("leader-locking-vpa-recommender")
		service                           = v.emptyService(recommender)
		deployment                        = v.emptyDeployment(recommender)
		podDisruptionBudget               = v.emptyPodDisruptionBudget(recommender)
		serviceMonitor                    = v.emptyServiceMonitor(recommender)
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
		{Obj: clusterRoleStatusActor, Class: component.Application, MutateFn: func() { v.reconcileRecommenderClusterRoleStatusActor(clusterRoleStatusActor) }},
		{Obj: clusterRoleBindingStatusActor, Class: component.Application, MutateFn: func() {
			v.reconcileRecommenderClusterRoleBinding(clusterRoleBindingStatusActor, clusterRoleStatusActor, recommender)
		}},
		{Obj: roleLeaderLocking, Class: component.Application, MutateFn: func() { v.reconcileRecommenderRoleLeaderLocking(roleLeaderLocking) }},
		{Obj: roleBindingLeaderLocking, Class: component.Application, MutateFn: func() {
			v.reconcileRecommenderRoleBindingLeaderLocking(roleBindingLeaderLocking, roleLeaderLocking, recommender)
		}},
		{Obj: service, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderService(service) }},
		{Obj: serviceMonitor, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderServiceMonitor(serviceMonitor) }},
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
			component.ResourceConfig{Obj: podDisruptionBudget, Class: component.Runtime, MutateFn: func() { v.reconcilePodDisruptionBudget(podDisruptionBudget, deployment) }},
		)
	} else {
		vpa := v.emptyVerticalPodAutoscaler(recommender)
		configs = append(configs,
			component.ResourceConfig{Obj: vpa, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderVPA(vpa, deployment) }},
			component.ResourceConfig{Obj: deployment, Class: component.Runtime, MutateFn: func() { v.reconcileRecommenderDeployment(deployment, nil) }},
			component.ResourceConfig{Obj: podDisruptionBudget, Class: component.Runtime, MutateFn: func() { v.reconcilePodDisruptionBudget(podDisruptionBudget, deployment) }},
		)
	}

	return configs
}

func (v *vpa) reconcileRecommenderServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = getRoleLabel()
	serviceAccount.AutomountServiceAccountToken = ptr.To(false)
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

func (v *vpa) reconcileRecommenderClusterRoleStatusActor(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers/status"},
			Verbs:     []string{"get", "patch"},
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
		Namespace: v.namespaceForApplicationClassResource(),
	}}
}

func (v *vpa) reconcileRecommenderRoleLeaderLocking(role *rbacv1.Role) {
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
			ResourceNames: []string{recommender},
			Verbs:         []string{"get", "watch", "update"},
		},
	}
}

func (v *vpa) reconcileRecommenderRoleBindingLeaderLocking(roleBinding *rbacv1.RoleBinding, role *rbacv1.Role, serviceAccountName string) {
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

func (v *vpa) reconcileRecommenderDeployment(deployment *appsv1.Deployment, serviceAccountName *string) {
	var cpuRequest string
	var memoryRequest string
	if v.values.ClusterType == component.ClusterTypeShoot {
		// The recommender in the shoot control plane is subject to autoscaling. Use small values and rely on the
		// autoscaler to adjust them.
		cpuRequest = "10m"
		memoryRequest = "15Mi"
	} else {
		// Seed recommenders are not subject to autoscaling. Use values which would suffice for large seeds and soil clusters
		cpuRequest = "200m"
		memoryRequest = "800M"
	}

	deployment.Labels = utils.MergeStringMaps(v.getDeploymentLabels(recommender), map[string]string{
		resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
	})
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             ptr.To(ptr.Deref(v.values.Recommender.Replicas, 1)),
		RevisionHistoryLimit: ptr.To[int32](2),
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
					Args:            v.computeRecommenderArgs(),
					LivenessProbe:   newDefaultLivenessProbe(),
					Ports: []corev1.ContainerPort{
						{
							Name:          serverPortName,
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

	v.injectAPIServerConnectionSpec(deployment, recommender, serviceAccountName)
}

func (v *vpa) reconcileRecommenderVPA(vpa *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) {
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

func (v *vpa) computeRecommenderArgs() []string {
	out := []string{
		"--v=3",
		"--stderrthreshold=info",
		"--pod-recommendation-min-cpu-millicores=5",
		"--pod-recommendation-min-memory-mb=10",
		fmt.Sprintf("--recommendation-margin-fraction=%f", ptr.Deref(v.values.Recommender.RecommendationMarginFraction, gardencorev1beta1.DefaultRecommendationMarginFraction)),
		fmt.Sprintf("--recommender-interval=%s", ptr.Deref(v.values.Recommender.Interval, gardencorev1beta1.DefaultRecommenderInterval).Duration),
		"--kube-api-qps=100",
		"--kube-api-burst=120",
		"--memory-saver=true",
		fmt.Sprintf("--target-cpu-percentile=%f", ptr.Deref(v.values.Recommender.TargetCPUPercentile, gardencorev1beta1.DefaultTargetCPUPercentile)),
		fmt.Sprintf("--recommendation-lower-bound-cpu-percentile=%f", ptr.Deref(v.values.Recommender.RecommendationLowerBoundCPUPercentile, gardencorev1beta1.DefaultRecommendationLowerBoundCPUPercentile)),
		fmt.Sprintf("--recommendation-upper-bound-cpu-percentile=%f", ptr.Deref(v.values.Recommender.RecommendationUpperBoundCPUPercentile, gardencorev1beta1.DefaultRecommendationUpperBoundCPUPercentile)),
		fmt.Sprintf("--cpu-histogram-decay-half-life=%s", ptr.Deref(v.values.Recommender.CPUHistogramDecayHalfLife, gardencorev1beta1.DefaultCPUHistogramDecayHalfLife).Duration),
		fmt.Sprintf("--target-memory-percentile=%f", ptr.Deref(v.values.Recommender.TargetMemoryPercentile, gardencorev1beta1.DefaultTargetMemoryPercentile)),
		fmt.Sprintf("--recommendation-lower-bound-memory-percentile=%f", ptr.Deref(v.values.Recommender.RecommendationLowerBoundMemoryPercentile, gardencorev1beta1.DefaultRecommendationLowerBoundMemoryPercentile)),
		fmt.Sprintf("--recommendation-upper-bound-memory-percentile=%f", ptr.Deref(v.values.Recommender.RecommendationUpperBoundMemoryPercentile, gardencorev1beta1.DefaultRecommendationUpperBoundMemoryPercentile)),
		fmt.Sprintf("--memory-histogram-decay-half-life=%s", ptr.Deref(v.values.Recommender.MemoryHistogramDecayHalfLife, gardencorev1beta1.DefaultMemoryHistogramDecayHalfLife).Duration),
		fmt.Sprintf("--memory-aggregation-interval=%s", ptr.Deref(v.values.Recommender.MemoryAggregationInterval, gardencorev1beta1.DefaultMemoryAggregationInterval).Duration),
		fmt.Sprintf("--memory-aggregation-interval-count=%d", ptr.Deref(v.values.Recommender.MemoryAggregationIntervalCount, gardencorev1beta1.DefaultMemoryAggregationIntervalCount)),
		"--leader-elect=true",
		fmt.Sprintf("--leader-elect-resource-namespace=%s", v.namespaceForApplicationClassResource()),
	}

	if v.values.ClusterType == component.ClusterTypeShoot {
		out = append(out, "--kubeconfig="+gardenerutils.PathGenericKubeconfig)
	}

	return out
}

func (v *vpa) reconcileRecommenderService(service *corev1.Service) {
	metricsNetworkPolicyPort := networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt32(recommenderPortMetrics)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}

	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		if v.values.IsGardenCluster {
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service, metricsNetworkPolicyPort))
		} else {
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, metricsNetworkPolicyPort))
		}
	case component.ClusterTypeShoot:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, metricsNetworkPolicyPort))
	}

	service.Labels = getAppLabel(recommender)
	service.Spec.Selector = getAppLabel(recommender)
	desiredPorts := []corev1.ServicePort{{
		Port:       recommenderPortMetrics,
		TargetPort: intstr.FromInt32(recommenderPortMetrics),
		Name:       metricsPortName,
	}}
	service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, "")
}

func (v *vpa) reconcileRecommenderServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor) {
	serviceMonitor.Labels = monitoringutils.Labels(v.getPrometheusLabel())
	serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
		Selector: metav1.LabelSelector{MatchLabels: getAppLabel(recommender)},
		Endpoints: []monitoringv1.Endpoint{{
			Port: metricsPortName,
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					Action:      "replace",
					Replacement: ptr.To(recommender),
					TargetLabel: "job",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_port_name"},
					Regex:        metricsPortName,
					Action:       "keep",
				},
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_pod_label_(.+)`,
				},
			},
		}},
	}
}
