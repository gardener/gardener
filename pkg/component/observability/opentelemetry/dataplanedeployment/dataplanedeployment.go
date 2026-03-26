// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dataplanedeployment

import (
	"context"
	// "strconv"
	"time"

	// monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	// resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	// kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	prometheusshoot "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	// "github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "shoot-core-otel-collector-dataplane"
	componentName       = "otel-collector-dataplane-deployment"

	labelKeyComponent = "component"
	metricsPortName   = "metrics"
	portMetrics       = int32(8080)

	targetNamespace = metav1.NamespaceSystem
	// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or deleted.
	TimeoutWaitForManagedResource = 2 * time.Minute
)

// KubernetesScrapeConfig defines a Kubernetes service discovery scrape job.
type KubernetesScrapeConfig struct {
	JobName    string
	Role       string
	Namespaces []string
}

// Config is the configuration for the OTEL collector dataplane deployment component.
type Config struct {
	// Image is the container image used for the OTEL collector.
	Image string
	// Replicas is the number of replicas for the deployment.
	Replicas int32
	// KubernetesScrapeConfigs defines the Kubernetes service discovery scrape jobs.
	KubernetesScrapeConfigs []KubernetesScrapeConfig
}

type dataplaneDeployment struct {
	client    client.Client
	namespace string
	config    Config
}

// New creates a new instance of DeployWaiter for OTEL collector dataplane deployment.
func New(
	client client.Client,
	namespace string,
	config Config,
) component.DeployWaiter {
	return &dataplaneDeployment{
		client:    client,
		namespace: namespace,
		config:    config,
	}
}

func (d *dataplaneDeployment) Deploy(ctx context.Context) error {
	data, err := d.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForShoot(ctx, d.client, d.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (d *dataplaneDeployment) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObjects(ctx, d.client,
		d.emptyScrapeConfig(),
	); err != nil {
		return err
	}

	return managedresources.DeleteForShoot(ctx, d.client, d.namespace, managedResourceName)
}

func (d *dataplaneDeployment) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, d.client, d.namespace, managedResourceName)
}

func (d *dataplaneDeployment) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, d.client, d.namespace, managedResourceName)
}

func (d *dataplaneDeployment) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta(componentName, d.namespace, prometheusshoot.Label)}
}

func (d *dataplaneDeployment) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(true),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   componentName,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes", "services", "endpoints", "pods"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes/metrics"},
					Verbs:     []string{"get"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   componentName,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				// ClusterIP: "None",
				Ports: []corev1.ServicePort{
					{
						Name:     metricsPortName,
						Port:     portMetrics,
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: getLabels(),
			},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels:    getLabels(),
			},
			Data: map[string]string{
				"config.yaml": d.getOTELConfig(),
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					// TODO: should it be GardenRoleMonitoring or GardenRoleObservability here?
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleMonitoring,
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				}),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             &d.config.Replicas,
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.GardenRole:                                                                v1beta1constants.GardenRoleMonitoring,
							managedresources.LabelKeyOrigin:                                                            managedresources.LabelValueGardener,
							v1beta1constants.LabelNetworkPolicyShootFromSeed:                                           v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootToAPIServer:                                        v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootToKubelet:                                          v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToDNS:               v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootToNodeExporter: v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:v1beta1constants.PriorityClassNameShootSystem700,
						ServiceAccountName: serviceAccount.Name,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65534),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            componentName,
								Image:           d.config.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/otelcol",
									"--config=/etc/otel-collector/config.yaml",
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          metricsPortName,
										ContainerPort: portMetrics,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									ReadOnlyRootFilesystem:   ptr.To(true),
									Capabilities: &corev1.Capabilities{
										Drop: []corev1.Capability{"ALL"},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config",
										MountPath: "/etc/otel-collector",
										ReadOnly:  true,
									},
									// {
									// 	Name:      "serviceaccount-token",
									// 	MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
									// 	ReadOnly:  true,
									// },
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
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
	)

	return registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		service,
		configMap,
		deployment,
	)
}

func (d *dataplaneDeployment) getOTELConfig() string {
	return otelConfig
}

func getLabels() map[string]string {
	return map[string]string{
		labelKeyComponent: componentName,
	}
}
