// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package victorialogs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	victoriametricsv1 "github.com/VictoriaMetrics/operator/api/operator/v1"
	victoriametricsv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	pvcautoscalerv1alpha1 "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/seed"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	timeoutWaitForManagedResources = 2 * time.Minute
)

// Values is the values for VictoriaLogs configurations.
type Values struct {
	// ImageRepository is the VictoriaLogs image repository.
	ImageRepository string
	// ImageTag is the VictoriaLogs image tag. May include an appended digest
	// in the form "<tag>@sha256:...".
	ImageTag string
	// Storage is the disk storage capacity of VictoriaLogs.
	// If not set, a default of 30Gi will be used.
	Storage *resource.Quantity
	// IsGardenCluster specifies whether VictoriaLogs is being deployed in a cluster registered as a Garden.
	IsGardenCluster bool
	// ClusterType is the type of the cluster where VictoriaLogs is deployed (Seed or Shoot).
	ClusterType component.ClusterType
	// Replicas is the number of VictoriaLogs replicas.
	Replicas int32
	// PriorityClassName is the name of the priority class for the VictoriaLogs pods.
	PriorityClassName string
	// PVCAutoscaler configures whether and how the VictoriaLogs PVC is autoscaled.
	PVCAutoscaling PVCAutoscalingConfig
}

// PVCAutoscalingConfig configures whether and up to what capacity the VictoriaLogs PVC is autoscaled.
type PVCAutoscalingConfig struct {
	// Enabled controls whether the component creates a PersistentVolumeClaimAutoscaler resource.
	Enabled bool
	// MaxCapacity is the upper bound up to which the PVC may be scaled.
	MaxCapacity resource.Quantity
}

type victoriaLogs struct {
	client    client.Client
	namespace string
	values    Values
}

// New creates a new instance of VictoriaLogs deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &victoriaLogs{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (v *victoriaLogs) Deploy(ctx context.Context) error {
	// TODO(rrhubenov): Remove this check once https://github.com/VictoriaMetrics/operator/pull/2401 is merged
	// and we update to the release that includes it. Until then, the VictoriaMetrics operator cannot handle a
	// digest-only image reference, so we reject it here. A digest-only tag has the form "<algorithm>:<hex>"
	// (e.g. "sha256:...", "sha512:...") whereas a tag+digest combined form contains a "@" separator.
	if !strings.Contains(v.values.ImageTag, "@") {
		for _, algorithmPrefix := range []string{"sha256:", "sha512:"} {
			if strings.HasPrefix(v.values.ImageTag, algorithmPrefix) {
				return fmt.Errorf("digest-only image reference %q is not supported yet", v.values.ImageRepository+"@"+v.values.ImageTag)
			}
		}
	}

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	resources := []client.Object{
		v.vlSingle(),
		v.getVPA(),
		v.getServiceMonitor(),
		v.getPrometheusRule(),
	}

	if v.values.PVCAutoscaling.Enabled {
		resources = append(resources, v.getPVCA(v.values.PVCAutoscaling))
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, v.client, v.namespace, constants.ManagedResourceNameRuntime, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (v *victoriaLogs) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, v.client, v.namespace, constants.ManagedResourceNameRuntime)
}

func (v *victoriaLogs) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, constants.ManagedResourceNameRuntime)
}

func (v *victoriaLogs) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, constants.ManagedResourceNameRuntime)
}

func (v *victoriaLogs) vlSingle() *victoriametricsv1.VLSingle {
	storage := resource.MustParse("30Gi")
	if v.values.Storage != nil {
		storage = *v.values.Storage
	}

	vlSingle := &victoriametricsv1.VLSingle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VLSingleResourceName,
			Namespace: v.namespace,
		},
		Spec: victoriametricsv1.VLSingleSpec{
			PodMetadata: &victoriametricsv1beta1.EmbeddedObjectMetadata{
				Labels: getLabels(),
			},
			CommonDefaultableParams: victoriametricsv1beta1.CommonDefaultableParams{
				DisableSelfServiceScrape: new(true),
				UseStrictSecurity:        new(true),
				Image: victoriametricsv1beta1.Image{
					Repository: v.values.ImageRepository,
					Tag:        v.values.ImageTag,
				},
				Port: strconv.Itoa(constants.VictoriaLogsPort),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100M"),
					},
				},
				UseDefaultResources: new(false),
			},
			CommonApplicationDeploymentParams: victoriametricsv1beta1.CommonApplicationDeploymentParams{
				ReplicaCount:      new(v.values.Replicas),
				PriorityClassName: v.values.PriorityClassName,
			},
			RetentionPeriod: "15d",
			Storage: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storage,
					},
				},
			},
			ServiceSpec: &victoriametricsv1beta1.AdditionalServiceSpec{
				EmbeddedObjectMetadata: victoriametricsv1beta1.EmbeddedObjectMetadata{
					Name: constants.ServiceName,
				},
			},
		},
	}

	// Add network policy annotations to allow Prometheus scraping based on cluster type.
	// ManagedMetadata propagates annotations to all objects created by the VM operator, including the Service.
	managedAnnotations := make(map[string]string)
	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		if v.values.IsGardenCluster {
			managedAnnotations[resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+v1beta1constants.LabelNetworkPolicyGardenScrapeTargets+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix] = fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, constants.VictoriaLogsPort)
		} else {
			managedAnnotations[resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+v1beta1constants.LabelNetworkPolicySeedScrapeTargets+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix] = fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, constants.VictoriaLogsPort)
		}
	case component.ClusterTypeShoot:
		managedAnnotations[resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+v1beta1constants.LabelNetworkPolicyScrapeTargets+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix] = fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, constants.VictoriaLogsPort)
		managedAnnotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias] = v1beta1constants.LabelNetworkPolicyShootNamespaceAlias
		managedAnnotations[resourcesv1alpha1.NetworkingNamespaceSelectors] = `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`
	}
	if len(managedAnnotations) > 0 {
		vlSingle.Spec.ManagedMetadata = &victoriametricsv1beta1.ManagedObjectsMetadata{
			Annotations: managedAnnotations,
		}
	}

	return vlSingle
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:                      constants.VLSingleResourceName,
		v1beta1constants.LabelRole:                     v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole:                    v1beta1constants.GardenRoleObservability,
		v1beta1constants.LabelObservabilityApplication: constants.VLSingleResourceName,
	}
}

func (v *victoriaLogs) getVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "victoria-logs-vpa",
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "vlsingle-" + constants.VLSingleResourceName,
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: new(vpaautoscalingv1.UpdateModeRecreate),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName:    "vlsingle",
						ControlledValues: new(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
				},
			},
		},
	}
}

func (v *victoriaLogs) getPVCA(pvcAutoscaling PVCAutoscalingConfig) *pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscaler {
	return &pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VLSingleResourceName,
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: pvcautoscalerv1alpha1.PersistentVolumeClaimAutoscalerSpec{
			TargetRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       "vlsingle-" + constants.VLSingleResourceName,
			},
			VolumePolicies: []pvcautoscalerv1alpha1.VolumePolicy{
				{
					MaxCapacity: pvcAutoscaling.MaxCapacity,
					ScaleUp: &pvcautoscalerv1alpha1.ScalingRules{
						UtilizationThresholdPercent: new(70),
						StepPercent:                 new(10),
						MinStepAbsolute:             new(resource.MustParse("1Gi")),
					},
				},
			},
		},
	}
}

func (v *victoriaLogs) getPrometheusLabel() string {
	if v.values.ClusterType == component.ClusterTypeSeed {
		if v.values.IsGardenCluster {
			return garden.Label
		}
		return seed.Label
	}
	return shoot.Label
}

func (v *victoriaLogs) getServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta("victoria-logs", v.namespace, v.getPrometheusLabel()),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name":      "vlsingle",
				"app.kubernetes.io/instance":  constants.VLSingleResourceName,
				"app.kubernetes.io/component": "monitoring",
				"managed-by":                  "vm-operator",
			}},
			Endpoints: []monitoringv1.Endpoint{{
				Port: "http",
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						Action:      "replace",
						Replacement: new("victoria-logs"),
						TargetLabel: "job",
					},
					{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					},
				},
			}},
		},
	}
}

func (v *victoriaLogs) getPrometheusRule() *monitoringv1.PrometheusRule {
	description := "There are no VictoriaLogs pods running on seed: {{ .ExternalLabels.seed }}. No logs will be collected."
	if v.values.ClusterType == component.ClusterTypeShoot {
		description = "There are no VictoriaLogs pods running. No logs will be collected."
	}

	return &monitoringv1.PrometheusRule{
		ObjectMeta: monitoringutils.ConfigObjectMeta("victoria-logs", v.namespace, v.getPrometheusLabel()),
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "victoria-logs.rules",
				Rules: []monitoringv1.Rule{{
					Alert: "VictoriaLogsDown",
					Expr:  intstr.FromString(`absent(up{job="victoria-logs"} == 1)`),
					For:   new(monitoringv1.Duration("30m")),
					Labels: map[string]string{
						"service":    "logging",
						"severity":   "warning",
						"type":       "seed",
						"visibility": "operator",
					},
					Annotations: map[string]string{
						"description": description,
						"summary":     "VictoriaLogs is down",
					},
				}},
			}},
		},
	}
}
