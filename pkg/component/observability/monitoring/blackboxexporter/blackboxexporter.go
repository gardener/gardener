// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package blackboxexporter

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	labelValue        = "blackbox-exporter"
	labelKeyComponent = "component"

	volumeMountPathConfig = "/etc/blackbox_exporter"
	dataKeyConfig         = "blackbox.yaml"

	volumeNameClusterAccess = "cluster-access"
	// VolumeMountPathClusterAccess is the volume mount path to the cluster access credentials.
	VolumeMountPathClusterAccess = "/var/run/secrets/blackbox_exporter/cluster-access"
)

// Interface contains functions for a blackbox-exporter deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
}

// Values is a set of configuration values for the blackbox-exporter.
type Values struct {
	// Image is the container image used for blackbox-exporter.
	Image string
	// ClusterType is the type of the cluster.
	ClusterType component.ClusterType
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the cluster.
	VPAEnabled bool
	// KubernetesVersion is the Kubernetes version of the cluster.
	KubernetesVersion *semver.Version
	// Config is blackbox exporter configuration.
	Config blackboxexporterconfig.Config
	// ScrapeConfigs is a list of scrape configs for the blackbox exporter.
	ScrapeConfigs []*monitoringv1alpha1.ScrapeConfig
	// PodLabels are additional labels for the pod.
	PodLabels map[string]string
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
}

// New creates a new instance of DeployWaiter for blackbox-exporter.
func New(
	client client.Client,
	secretsManager secretsmanager.Interface,
	namespace string,
	values Values,
) Interface {
	return &blackboxExporter{
		client:         client,
		secretsManager: secretsManager,
		namespace:      namespace,
		values:         values,
	}
}

type blackboxExporter struct {
	client         client.Client
	secretsManager secretsmanager.Interface
	namespace      string
	values         Values
}

func (b *blackboxExporter) Deploy(ctx context.Context) error {
	data, err := b.computeResourcesData()
	if err != nil {
		return err
	}

	if b.values.ClusterType == component.ClusterTypeSeed {
		return managedresources.CreateForSeedWithLabels(ctx, b.client, b.namespace, b.managedResourceName(), false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, data)
	}
	return managedresources.CreateForShoot(ctx, b.client, b.namespace, b.managedResourceName(), managedresources.LabelValueGardener, false, data)
}

func (b *blackboxExporter) Destroy(ctx context.Context) error {
	if b.values.ClusterType == component.ClusterTypeSeed {
		return managedresources.DeleteForSeed(ctx, b.client, b.namespace, b.managedResourceName())
	}
	return managedresources.DeleteForShoot(ctx, b.client, b.namespace, b.managedResourceName())
}

func (b *blackboxExporter) managedResourceName() string {
	if b.values.ClusterType == component.ClusterTypeSeed {
		return "blackbox-exporter"
	}
	return "shoot-core-blackbox-exporter"
}

func (b *blackboxExporter) namespaceName() string {
	if b.values.ClusterType == component.ClusterTypeSeed {
		return b.namespace
	}
	return metav1.NamespaceSystem
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *blackboxExporter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, b.managedResourceName())
}

func (b *blackboxExporter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, b.managedResourceName())
}

func (b *blackboxExporter) computeResourcesData() (map[string][]byte, error) {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	if b.values.ClusterType == component.ClusterTypeShoot {
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	}

	configRaw, err := yaml.Marshal(&b.values.Config)
	if err != nil {
		return nil, err
	}

	var (
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: b.namespaceName(),
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter-config",
				Namespace: b.namespaceName(),
				Labels: map[string]string{
					v1beta1constants.LabelApp:  "prometheus",
					v1beta1constants.LabelRole: v1beta1constants.GardenRoleMonitoring,
				},
			},
			Data: map[string]string{dataKeyConfig: string(configRaw)},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: b.namespaceName(),
				Labels:    utils.MergeStringMaps(getLabels(), map[string]string{resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer}),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector:             &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyComponent: labelValue}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
						Labels:      utils.MergeStringMaps(getLabels(), b.values.PodLabels),
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						PriorityClassName:  b.values.PriorityClassName,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:          ptr.To[int64](65534),
							FSGroup:            ptr.To[int64](65534),
							SupplementalGroups: []int64{1},
							SeccompProfile:     &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
						},
						Containers: []corev1.Container{
							{
								Name:  "blackbox-exporter",
								Image: b.values.Image,
								Args: []string{
									fmt.Sprintf("--config.file=%s/%s", volumeMountPathConfig, dataKeyConfig),
									"--log.level=debug",
								},
								ImagePullPolicy: corev1.PullIfNotPresent,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "probe",
										ContainerPort: int32(9115),
										Protocol:      corev1.ProtocolTCP,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "blackbox-exporter-config",
										MountPath: volumeMountPathConfig,
									},
								},
							},
						},
						DNSConfig: &corev1.PodDNSConfig{
							Options: []corev1.PodDNSConfigOption{
								{
									Name:  "ndots",
									Value: ptr.To("3"),
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "blackbox-exporter-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: b.namespaceName(),
				Labels: map[string]string{
					labelKeyComponent: labelValue,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:     "probe",
						Port:     int32(9115),
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: map[string]string{
					labelKeyComponent: labelValue,
				},
			},
		}

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: b.namespaceName(),
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
					labelKeyComponent:           labelValue,
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt32(1),
				Selector:       deployment.Spec.Selector,
			},
		}

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	utilruntime.Must(references.InjectAnnotations(deployment))
	kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, b.values.KubernetesVersion)

	if b.values.VPAEnabled {
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: b.namespaceName(),
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						},
					},
				},
			},
		}
	}

	for _, scrapeConfig := range b.values.ScrapeConfigs {
		if err := registry.Add(scrapeConfig); err != nil {
			return nil, err
		}
	}

	if b.values.ClusterType == component.ClusterTypeSeed {
		caSecret, found := b.secretsManager.Get(v1beta1constants.SecretNameCACluster)
		if !found {
			return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
		}

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameClusterAccess,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To(int32(420)),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: caSecret.Name},
								Items: []corev1.KeyToPath{{
									Key:  secrets.DataKeyCertificateBundle,
									Path: secrets.DataKeyCertificateBundle,
								}},
								Optional: ptr.To(false),
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: gardenprometheus.AccessSecretName},
								Items: []corev1.KeyToPath{{
									Key:  resourcesv1alpha1.DataKeyToken,
									Path: resourcesv1alpha1.DataKeyToken,
								}},
								Optional: ptr.To(false),
							},
						},
					},
				},
			},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameClusterAccess,
			MountPath: VolumeMountPathClusterAccess,
		})
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		configMap,
		deployment,
		podDisruptionBudget,
		service,
		vpa,
	)
}

func getLabels() map[string]string {
	return map[string]string{
		labelKeyComponent:               labelValue,
		v1beta1constants.GardenRole:     v1beta1constants.GardenRoleMonitoring,
		managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
	}
}
