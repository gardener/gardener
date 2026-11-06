// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricscollector

import (
	"fmt"
	"strconv"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	metricsPortName = "metrics"

	kubeconfigVolumeName = "kubeconfig"
)

func (g *gardenerMetricsCollector) openTelemetryCollector(genericTokenKubeconfigSecretName, virtualGardenAccessSecretName string) *otelv1beta1.OpenTelemetryCollector {
	obj := &otelv1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openTelemetryCollectorName,
			Namespace: g.namespace,
			Labels:    getLabels(),
		},
		Spec: otelv1beta1.OpenTelemetryCollectorSpec{
			Mode:            "deployment",
			UpgradeStrategy: "none",
			Observability: otelv1beta1.ObservabilitySpec{
				Metrics: otelv1beta1.MetricsConfigSpec{
					DisablePrometheusAnnotations: true,
				},
			},
			OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
				Image:             g.values.Image,
				Replicas:          new(int32(2)),
				PriorityClassName: v1beta1constants.PriorityClassNameGardenSystem100,
				ServiceAccount:    openTelemetryCollectorName,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: new(false),
					ReadOnlyRootFilesystem:   new(true),
				},
				Volumes: []corev1.Volume{
					gardenerutils.GenerateGenericKubeconfigVolume(genericTokenKubeconfigSecretName, virtualGardenAccessSecretName, kubeconfigVolumeName),
				},
				VolumeMounts: []corev1.VolumeMount{
					gardenerutils.GenerateGenericKubeconfigVolumeMount(kubeconfigVolumeName, gardenerutils.VolumeMountPathGenericKubeconfig),
				},
				Ports: []otelv1beta1.PortsSpec{{
					ServicePort: corev1.ServicePort{
						Name:     metricsPortName,
						Port:     metricsPort,
						Protocol: corev1.ProtocolTCP,
					},
				}},
			},
			Config: otelv1beta1.Config{
				Receivers: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"gardener": map[string]any{
							"kubeconfig": gardenerutils.PathGenericKubeconfig},
					},
				},
				Exporters: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"prometheus": map[string]any{
							"endpoint": "[::]:" + strconv.Itoa(metricsPort),
						},
					},
				},
				Service: otelv1beta1.Service{
					Telemetry: &otelv1beta1.AnyConfig{
						Object: map[string]any{
							"metrics": map[string]any{
								"level": "basic",
								"readers": []any{
									map[string]any{
										"pull": map[string]any{
											"exporter": map[string]any{
												"prometheus": map[string]any{
													"host": "[::]",
													"port": internalMetricsPort,
												},
											},
										},
									},
								},
							},
						},
					},
					Pipelines: map[string]*otelv1beta1.Pipeline{
						"metrics": {
							Receivers: []string{"gardener"},
							Exporters: []string{"prometheus"},
						},
					},
				},
			},
		},
	}

	metav1.SetMetaDataLabel(&obj.ObjectMeta, gardenerutils.NetworkPolicyLabel(operatorv1alpha1.DeploymentNameVirtualGardenKubeAPIServer, kubeapiserverconstants.Port), v1beta1constants.LabelNetworkPolicyAllowed)
	metav1.SetMetaDataLabel(&obj.ObjectMeta, v1beta1constants.LabelNetworkPolicyToDNS, v1beta1constants.LabelNetworkPolicyAllowed)

	metav1.SetMetaDataAnnotation(&obj.ObjectMeta, resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix+v1beta1constants.LabelNetworkPolicyGardenScrapeTargets+resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix, fmt.Sprintf(`[{"protocol":"TCP","port":%d},{"protocol":"TCP","port":%d}]`, metricsPort, internalMetricsPort))

	return obj
}
