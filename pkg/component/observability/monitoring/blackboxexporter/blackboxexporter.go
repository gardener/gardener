// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package blackboxexporter

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	shootprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	labelValue = "blackbox-exporter"

	volumeMountPathConfig = "/etc/blackbox_exporter"
	dataKeyConfig         = "blackbox.yaml"

	volumeNameClusterAccess = "cluster-access"
	// VolumeMountPathClusterAccess is the volume mount path to the cluster access credentials.
	VolumeMountPathClusterAccess = "/var/run/secrets/blackbox_exporter/cluster-access"

	volumeNameGardenerCA = "gardener-ca"
	// VolumeMountPathGardenerCA is the volume mount path to the gardener CA certificate bundle.
	VolumeMountPathGardenerCA = "/var/run/secrets/blackbox_exporter/gardener-ca"

	port int32 = 9115
)

// Values is a set of configuration values for the blackbox-exporter.
type Values struct {
	// Image is the container image used for blackbox-exporter.
	Image string
	// ClusterType is the type of the cluster.
	ClusterType component.ClusterType
	// IsGardenCluster is specifying whether the component is deployed to the garden cluster.
	IsGardenCluster bool
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the cluster.
	VPAEnabled bool
	// KubernetesVersion is the Kubernetes version of the cluster.
	KubernetesVersion *semver.Version
	// Config is blackbox exporter configuration.
	Config blackboxexporterconfig.Config
	// ScrapeConfigs is a list of scrape configs for the blackbox exporter.
	ScrapeConfigs []*monitoringv1alpha1.ScrapeConfig
	// PrometheusRules is a list of PrometheusRules for the blackbox exporter.
	PrometheusRules []*monitoringv1.PrometheusRule
	// PodLabels are additional labels for the pod.
	PodLabels map[string]string
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas is the number of replicas
	Replicas int32
}

// New creates a new instance of DeployWaiter for blackbox-exporter.
func New(
	client client.Client,
	secretsManager secretsmanager.Interface,
	namespace string,
	values Values,
) component.DeployWaiter {
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

	for _, scrapeConfig := range b.values.ScrapeConfigs {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.client, scrapeConfig, func() error { return nil }); err != nil {
			return err
		}
	}

	for _, prometheusRule := range b.values.PrometheusRules {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.client, prometheusRule, func() error { return nil }); err != nil {
			return err
		}
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

func (b *blackboxExporter) runtimeNamespace() string {
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
				Namespace: b.runtimeNamespace(),
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter-config",
				Namespace: b.runtimeNamespace(),
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
				Namespace: b.runtimeNamespace(),
				Labels:    utils.MergeStringMaps(getLabels(), map[string]string{resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer}),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             &b.values.Replicas,
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector:             &metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelApp: labelValue}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
						Labels:      utils.MergeStringMaps(getLabels(), b.values.PodLabels),
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						PriorityClassName:  b.values.PriorityClassName,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot:       ptr.To(true),
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
										corev1.ResourceMemory: resource.MustParse("15M"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "probe",
										ContainerPort: port,
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
				Namespace: b.runtimeNamespace(),
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelValue,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:     "probe",
						Port:     port,
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: map[string]string{
					v1beta1constants.LabelApp: labelValue,
				},
			},
		}

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: b.runtimeNamespace(),
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
					v1beta1constants.LabelApp:   labelValue,
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				Selector:       deployment.Spec.Selector,
			},
		}

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	utilruntime.Must(references.InjectAnnotations(deployment))
	kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, b.values.KubernetesVersion)

	if b.values.ClusterType == component.ClusterTypeSeed {
		networkPolicyPort := networkingv1.NetworkPolicyPort{
			Port:     ptr.To(intstr.FromInt32(port)),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}

		if b.values.IsGardenCluster {
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service, networkPolicyPort))
		} else {
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkPolicyPort))
		}
	}

	if b.values.VPAEnabled {
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: b.runtimeNamespace(),
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
							ContainerName:       vpaautoscalingv1.DefaultContainerResourcePolicy,
							ControlledValues:    ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							ControlledResources: &[]corev1.ResourceName{corev1.ResourceMemory},
						},
					},
				},
			},
		}
	}

	if b.values.ClusterType == component.ClusterTypeSeed {
		for _, scrapeConfig := range b.values.ScrapeConfigs {
			if err := registry.Add(scrapeConfig); err != nil {
				return nil, err
			}
		}

		for _, prometheusRule := range b.values.PrometheusRules {
			if err := registry.Add(prometheusRule); err != nil {
				return nil, err
			}
		}
	}

	if b.values.ClusterType == component.ClusterTypeSeed {
		caSecret, found := b.secretsManager.Get(v1beta1constants.SecretNameCACluster)
		if !found {
			return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
		}

		accessSecretName := shootprometheus.AccessSecretName
		if b.values.IsGardenCluster {
			accessSecretName = gardenprometheus.AccessSecretName
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
								LocalObjectReference: corev1.LocalObjectReference{Name: accessSecretName},
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

		if b.values.IsGardenCluster {
			caGardenerSecret, found := b.secretsManager.Get(operatorv1alpha1.SecretNameCAGardener)
			if !found {
				return nil, fmt.Errorf("secret %q not found", operatorv1alpha1.SecretNameCAGardener)
			}

			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeNameGardenerCA,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: caGardenerSecret.Name,
					},
				},
			})
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      volumeNameGardenerCA,
				MountPath: VolumeMountPathGardenerCA,
			})
		}
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
		v1beta1constants.LabelApp:       labelValue,
		v1beta1constants.GardenRole:     v1beta1constants.GardenRoleMonitoring,
		managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
	}
}
