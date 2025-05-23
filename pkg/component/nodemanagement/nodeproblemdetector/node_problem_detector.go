// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeproblemdetector

import (
	"context"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName                          = "shoot-core-node-problem-detector"
	serviceAccountName                           = "node-problem-detector"
	serviceName                                  = "node-problem-detector"
	containerName                                = "node-problem-detector"
	daemonSetName                                = "node-problem-detector"
	clusterRoleName                              = "node-problem-detector"
	clusterRoleBindingName                       = "node-problem-detector"
	vpaName                                      = "node-problem-detector"
	daemonSetTerminationGracePeriodSeconds int64 = 30
	daemonSetPrometheusPort                      = 20257
	labelValue                                   = "node-problem-detector"
)

// Values is a set of configuration values for the node-problem-detector component.
type Values struct {
	// APIServerHost is the host of the kube-apiserver.
	APIServerHost *string
	// Image is the container image used for node-problem-detector.
	Image string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// KubernetesVersion is the Kubernetes version of the Shoot.
	KubernetesVersion *semver.Version
}

// New creates a new instance of DeployWaiter for node-problem-detector.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &nodeProblemDetector{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nodeProblemDetector struct {
	client    client.Client
	namespace string
	values    Values
}

func (c *nodeProblemDetector) Deploy(ctx context.Context) error {
	data, err := c.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, c.client, c.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
}

func (c *nodeProblemDetector) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, c.client, c.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *nodeProblemDetector) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *nodeProblemDetector) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *nodeProblemDetector) computeResourcesData() (map[string][]byte, error) {
	var (
		registry             = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
		hostPathFileOrCreate = corev1.HostPathFileOrCreate
		serviceAccount       = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterRoleName,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes/status"},
					Verbs:     []string{"patch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch", "update"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        clusterRoleBindingName,
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
				Labels:      getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonSetName,
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
				}),
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: utils.MergeStringMaps(getLabels(), map[string]string{
						v1beta1constants.LabelApp: labelValue,
					}),
				},
				RevisionHistoryLimit: ptr.To[int32](2),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.LabelApp:                           labelValue,
							v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleSystemComponent,
							v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
							managedresources.LabelKeyOrigin:                     managedresources.LabelValueGardener,
						}),
					},
					Spec: corev1.PodSpec{
						DNSPolicy:                     corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
						ServiceAccountName:            serviceAccount.Name,
						HostNetwork:                   false,
						TerminationGracePeriodSeconds: ptr.To(daemonSetTerminationGracePeriodSeconds),
						PriorityClassName:             v1beta1constants.PriorityClassNameShootSystem900,
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            daemonSetName,
								Image:           c.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/sh",
									"-c",
									"exec /node-problem-detector --logtostderr --config.system-log-monitor=/config/kernel-monitor.json,/config/docker-monitor.json,/config/systemd-monitor.json,/config/readonly-monitor.json .. --config.custom-plugin-monitor=/config/kernel-monitor-counter.json,/config/systemd-monitor-counter.json .. --config.system-stats-monitor=/config/system-stats-monitor.json --prometheus-port=" + strconv.Itoa(daemonSetPrometheusPort),
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: ptr.To(true),
								},
								Env: []corev1.EnvVar{
									{
										Name: "NODE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "spec.nodeName",
											},
										},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "log",
										MountPath: "/var/log/journal",
										ReadOnly:  true,
									},
									{
										Name:      "localtime",
										MountPath: "/etc/localtime",
										ReadOnly:  true,
									},
									{
										Name:      "kmsg",
										MountPath: "/dev/kmsg",
										ReadOnly:  true,
									},
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "exporter",
										ContainerPort: int32(daemonSetPrometheusPort),
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20m"),
										corev1.ResourceMemory: resource.MustParse("20Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("500Mi"),
									},
								},
							},
						},
						Tolerations: []corev1.Toleration{
							{
								Effect:   corev1.TaintEffectNoSchedule,
								Operator: corev1.TolerationOpExists,
							},
							{
								Effect:   corev1.TaintEffectNoExecute,
								Operator: corev1.TolerationOpExists,
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "log",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/log/journal",
									},
								},
							},
							{
								Name: "localtime",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/etc/localtime",
										Type: &hostPathFileOrCreate,
									},
								},
							},
							{
								Name: "kmsg",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/dev/kmsg",
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
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.LabelApp:   labelValue,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent,
				}),
			},
			Spec: corev1.ServiceSpec{
				Selector: getLabels(),
				Ports: []corev1.ServicePort{
					{
						Port:       int32(daemonSetPrometheusPort),
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(daemonSetPrometheusPort),
					},
				},
			},
		}

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	if c.values.VPAEnabled {
		updateMode := vpaautoscalingv1.UpdateModeAuto
		controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vpaName,
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "DaemonSet",
					Name:       daemonSet.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &updateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
							ControlledValues: &controlledValues,
						},
					},
				},
			},
		}
	}

	if c.values.APIServerHost != nil {
		daemonSet.Spec.Template.Spec.Containers[0].Env = append(daemonSet.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_HOST",
			Value: *c.values.APIServerHost,
		})
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		daemonSet,
		service,
		vpa,
	)
}

func getLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     labelValue,
		"app.kubernetes.io/instance": "shoot-core",
	}
}
