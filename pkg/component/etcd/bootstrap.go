// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheus/cache"
	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// Druid is a constant for the name of the etcd-druid.
	Druid = "etcd-druid"

	druidRBACName                                = "gardener.cloud:system:" + Druid
	druidServiceAccountName                      = Druid
	druidVPAName                                 = Druid + "-vpa"
	druidConfigMapImageVectorOverwriteNamePrefix = Druid + "-imagevector-overwrite"
	druidServiceName                             = Druid
	druidDeploymentName                          = Druid
	managedResourceControlName                   = Druid

	metricsPortName = "metrics"
	metricsPort     = 8080

	druidConfigMapImageVectorOverwriteDataKey          = "images_overwrite.yaml"
	druidDeploymentVolumeMountPathImageVectorOverwrite = "/charts_overwrite"
	druidDeploymentVolumeNameImageVectorOverwrite      = "imagevector-overwrite"
)

// NewBootstrapper creates a new instance of DeployWaiter for the etcd bootstrapper.
func NewBootstrapper(
	c client.Client,
	namespace string,
	kubernetesVersion *semver.Version,
	etcdConfig *config.ETCDConfig,
	image string,
	imageVectorOverwrite *string,
	priorityClassName string,
) component.DeployWaiter {
	return &bootstrapper{
		client:               c,
		namespace:            namespace,
		kubernetesVersion:    kubernetesVersion,
		etcdConfig:           etcdConfig,
		image:                image,
		imageVectorOverwrite: imageVectorOverwrite,
		priorityClassName:    priorityClassName,
	}
}

type bootstrapper struct {
	client               client.Client
	namespace            string
	kubernetesVersion    *semver.Version
	etcdConfig           *config.ETCDConfig
	image                string
	imageVectorOverwrite *string
	priorityClassName    string
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		labels   = func() map[string]string { return map[string]string{v1beta1constants.GardenRole: Druid} }

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidServiceAccountName,
				Namespace: b.namespace,
				Labels:    labels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   druidRBACName,
				Labels: labels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list", "watch", "delete", "deletecollection"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"secrets", "endpoints"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"events"},
					Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"serviceaccounts"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{rbacv1.GroupName},
					Resources: []string{"roles", "rolebindings"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"services", "configmaps"},
					Verbs:     []string{"get", "list", "patch", "update", "watch", "create", "delete"},
				},
				{
					APIGroups: []string{appsv1.GroupName},
					Resources: []string{"statefulsets"},
					Verbs:     []string{"get", "list", "patch", "update", "watch", "create", "delete"},
				},
				{
					APIGroups: []string{batchv1.GroupName},
					Resources: []string{"jobs"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{druidv1alpha1.GroupVersion.Group},
					Resources: []string{"etcds", "etcdcopybackupstasks"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{druidv1alpha1.GroupVersion.Group},
					Resources: []string{"etcds/status", "etcds/finalizers", "etcdcopybackupstasks/status", "etcdcopybackupstasks/finalizers"},
					Verbs:     []string{"get", "update", "patch", "create"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"persistentvolumeclaims"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{policyv1beta1.GroupName},
					Resources: []string{"poddisruptionbudgets"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   druidRBACName,
				Labels: labels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     druidRBACName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      druidServiceAccountName,
					Namespace: b.namespace,
				},
			},
		}

		configMapImageVectorOverwrite = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidConfigMapImageVectorOverwriteNamePrefix,
				Namespace: b.namespace,
				Labels:    labels(),
			},
		}

		vpaUpdateMode = vpaautoscalingv1.UpdateModeAuto
		vpa           = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidVPAName,
				Namespace: b.namespace,
				Labels:    labels(),
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       druidDeploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("100M"),
						},
					}},
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidServiceName,
				Namespace: b.namespace,
				Labels: utils.MergeStringMaps(map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				}, labels()),
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: labels(),
				Ports: []corev1.ServicePort{
					{
						Name:       metricsPortName,
						Protocol:   corev1.ProtocolTCP,
						Port:       metricsPort,
						TargetPort: intstr.FromInt32(metricsPort),
					},
				},
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidDeploymentName,
				Namespace: b.namespace,
				Labels: utils.MergeStringMaps(map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				}, labels()),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To(int32(1)),
				RevisionHistoryLimit: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: labels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(labels(), map[string]string{
							v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  b.priorityClassName,
						ServiceAccountName: druidServiceAccountName,
						Containers: []corev1.Container{
							{
								Name:            Druid,
								Image:           b.image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         getDruidDeployCommands(b.etcdConfig),
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
								Ports: []corev1.ContainerPort{{
									ContainerPort: metricsPort,
								}},
							},
						},
					},
				},
			},
		}

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidDeploymentName,
				Namespace: deployment.Namespace,
				Labels:    labels(),
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt32(1),
				Selector:       deployment.Spec.Selector,
			},
		}

		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: monitoringutils.ConfigObjectMeta(druidServiceName, b.namespace, cache.Label),
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: labels()},
				Endpoints: []monitoringv1.Endpoint{{
					Port: metricsPortName,
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"etcddruid_compaction_jobs_total",
						"etcddruid_compaction_jobs_current",
						"etcddruid_compaction_job_duration_seconds_bucket",
						"etcddruid_compaction_job_duration_seconds_sum",
						"etcddruid_compaction_job_duration_seconds_count",
						"etcddruid_compaction_num_delta_events",
					),
				}},
			},
		}

		resourcesToAdd = []client.Object{
			serviceAccount,
			clusterRole,
			clusterRoleBinding,
			vpa,
			serviceMonitor,
		}
	)

	kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, b.kubernetesVersion)

	resourcesToAdd = append(resourcesToAdd, podDisruptionBudget)

	portMetrics := networkingv1.NetworkPolicyPort{
		Port:     utils.IntStrPtrFromInt32(metricsPort),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, portMetrics))

	resourcesToAdd = append(resourcesToAdd, service)

	if b.imageVectorOverwrite != nil {
		configMapImageVectorOverwrite.Data = map[string]string{druidConfigMapImageVectorOverwriteDataKey: *b.imageVectorOverwrite}
		utilruntime.Must(kubernetesutils.MakeUnique(configMapImageVectorOverwrite))
		resourcesToAdd = append(resourcesToAdd, configMapImageVectorOverwrite)

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: druidDeploymentVolumeNameImageVectorOverwrite,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapImageVectorOverwrite.Name,
					},
				},
			},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      druidDeploymentVolumeNameImageVectorOverwrite,
			MountPath: druidDeploymentVolumeMountPathImageVectorOverwrite,
			ReadOnly:  true,
		})
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  imagevector.OverrideEnv,
			Value: druidDeploymentVolumeMountPathImageVectorOverwrite + "/" + druidConfigMapImageVectorOverwriteDataKey,
		})

		utilruntime.Must(references.InjectAnnotations(deployment))
	}

	resources, err := registry.AddAllAndSerialize(append(resourcesToAdd, deployment)...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, b.client, b.namespace, managedResourceControlName, false, resources)
}

func getDruidDeployCommands(etcdConfig *config.ETCDConfig) []string {
	command := []string{
		"/etcd-druid",
		"--enable-leader-election=true",
		"--ignore-operation-annotation=false",
		"--disable-etcd-serviceaccount-automount=true",
		"--workers=" + strconv.FormatInt(*etcdConfig.ETCDController.Workers, 10),
		"--custodian-workers=" + strconv.FormatInt(*etcdConfig.CustodianController.Workers, 10),
		"--compaction-workers=" + strconv.FormatInt(*etcdConfig.BackupCompactionController.Workers, 10),
		"--enable-backup-compaction=" + strconv.FormatBool(*etcdConfig.BackupCompactionController.EnableBackupCompaction),
		"--etcd-events-threshold=" + strconv.FormatInt(*etcdConfig.BackupCompactionController.EventsThreshold, 10),
	}

	if etcdConfig.BackupCompactionController.MetricsScrapeWaitDuration != nil {
		command = append(command, "--metrics-scrape-wait-duration="+etcdConfig.BackupCompactionController.MetricsScrapeWaitDuration.Duration.String())
	}

	if etcdConfig.BackupCompactionController.ActiveDeadlineDuration != nil {
		command = append(command, "--active-deadline-duration="+etcdConfig.BackupCompactionController.ActiveDeadlineDuration.Duration.String())
	}

	// Add feature gates to the etcd druid command
	if etcdConfig.FeatureGates != nil {
		command = append(command, kubernetesutils.FeatureGatesToCommandLineParameter(etcdConfig.FeatureGates))
	}

	return command
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	etcdList := &druidv1alpha1.EtcdList{}
	// Need to check for both error types. The DynamicRestMapper can hold a stale cache returning a path to a non-existing api-resource leading to a NotFound error.
	if err := b.client.List(ctx, etcdList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdList.Items) > 0 {
		return fmt.Errorf("cannot debootstrap etcd-druid because there are still druidv1alpha1.Etcd resources left in the cluster")
	}

	etcdCopyBackupsTaskList := &druidv1alpha1.EtcdCopyBackupsTaskList{}
	if err := b.client.List(ctx, etcdCopyBackupsTaskList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdCopyBackupsTaskList.Items) > 0 {
		return fmt.Errorf("cannot debootstrap etcd-druid because there are still druidv1alpha1.EtcdCopyBackupsTask resources left in the cluster")
	}

	return managedresources.DeleteForSeed(ctx, b.client, b.namespace, managedResourceControlName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *bootstrapper) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthyAndNotProgressing(timeoutCtx, b.client, b.namespace, managedResourceControlName)
}

func (b *bootstrapper) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, managedResourceControlName)
}
