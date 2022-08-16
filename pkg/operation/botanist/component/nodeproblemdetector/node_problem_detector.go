// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodeproblemdetector

import (
	"context"
	"strconv"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName                    = "shoot-core-node-problem-detector"
	serviceAccountName                     = "node-problem-detector"
	deploymentName                         = "node-problem-detector"
	containerName                          = "node-problem-detector"
	daemonSetName                          = "node-problem-detector"
	clusterRoleName                        = "node-problem-detector"
	clusterRoleBindingName                 = "node-problem-detector"
	clusterRolePSPName                     = "gardener.cloud:psp:kube-system:node-problem-detector"
	clusterRoleBindingPSPName              = "gardener.cloud:psp:node-problem-detector"
	vpaName                                = "node-problem-detector"
	daemonSetTerminationGracePeriodSeconds = 30
	daemonSetPrometheusPort                = 20257
	daemonSetPrometheusAddress             = "0.0.0.0"
	podSecurityPolicyName                  = "node-problem-detector"
	labelValue                             = "node-problem-detector"
)

// Values is a set of configuration values for the node-problem-detector component.
type Values struct {
	// APIServerHost is the host of the kube-apiserver.
	APIServerHost *string
	// Image is the container image used for node-problem-detector.
	Image string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// PSPDisabled marks whether the PodSecurityPolicy admission plugin is disabled.
	PSPDisabled bool
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

	return managedresources.CreateForShoot(ctx, c.client, c.namespace, ManagedResourceName, false, data)
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
			AutomountServiceAccountToken: pointer.Bool(false),
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
				Labels: map[string]string{
					"app.kubernetes.io/instance":    "shoot-core",
					"app.kubernetes.io/name":        labelValue,
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
				},
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.LabelApp:    labelValue,
						"app.kubernetes.io/instance": "shoot-core",
						"app.kubernetes.io/name":     labelValue,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1beta1constants.LabelApp:                           labelValue,
							"app.kubernetes.io/instance":                        "shoot-core",
							"app.kubernetes.io/name":                            labelValue,
							v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleSystemComponent,
							v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
							managedresources.LabelKeyOrigin:                     managedresources.LabelValueGardener,
						},
					},
					Spec: corev1.PodSpec{
						DNSPolicy:                     corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
						ServiceAccountName:            serviceAccount.Name,
						HostNetwork:                   false,
						TerminationGracePeriodSeconds: pointer.Int64(daemonSetTerminationGracePeriodSeconds),
						PriorityClassName:             "system-cluster-critical",
						Containers: []corev1.Container{
							{
								Name:            daemonSetName,
								Image:           c.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/sh",
									"-c",
									"exec /node-problem-detector --logtostderr --config.system-log-monitor=/config/kernel-monitor.json,/config/docker-monitor.json,/config/systemd-monitor.json .. --config.custom-plugin-monitor=/config/kernel-monitor-counter.json,/config/systemd-monitor-counter.json .. --config.system-stats-monitor=/config/system-stats-monitor.json --prometheus-address=" + daemonSetPrometheusAddress + " --prometheus-port=" + strconv.Itoa(daemonSetPrometheusPort),
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: pointer.Bool(true),
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
										MountPath: "/var/log",
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
									Limits: c.getResourceLimits(),
								},
							},
						},
						Tolerations: []corev1.Toleration{
							{
								Effect:   corev1.TaintEffectNoSchedule,
								Operator: corev1.TolerationOpExists,
							},
							{
								Key:      "CriticalAddonsOnly",
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
										Path: "/var/log/",
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

		vpa                   *vpaautoscalingv1.VerticalPodAutoscaler
		podSecurityPolicy     *policyv1beta1.PodSecurityPolicy
		clusterRolePSP        *rbacv1.ClusterRole
		clusterRoleBindingPSP *rbacv1.ClusterRoleBinding
	)

	if !c.values.PSPDisabled {
		podSecurityPolicy = &policyv1beta1.PodSecurityPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: podSecurityPolicyName,
				Annotations: map[string]string{
					v1beta1constants.AnnotationSeccompAllowedProfiles: v1beta1constants.AnnotationSeccompAllowedProfilesRuntimeDefaultValue,
					v1beta1constants.AnnotationSeccompDefaultProfile:  v1beta1constants.AnnotationSeccompAllowedProfilesRuntimeDefaultValue,
				},
				Labels: getLabels(),
			},
			Spec: policyv1beta1.PodSecurityPolicySpec{
				Privileged:               true,
				AllowPrivilegeEscalation: pointer.Bool(true),
				AllowedCapabilities: []corev1.Capability{
					corev1.Capability(policyv1beta1.All),
				},
				Volumes: []policyv1beta1.FSType{
					policyv1beta1.ConfigMap,
					policyv1beta1.EmptyDir,
					policyv1beta1.Projected,
					policyv1beta1.Secret,
					policyv1beta1.DownwardAPI,
					policyv1beta1.HostPath,
				},
				HostNetwork: false,
				HostIPC:     false,
				HostPID:     false,
				AllowedHostPaths: []policyv1beta1.AllowedHostPath{
					{PathPrefix: "/etc/localtime"},
					{PathPrefix: "/var/log"},
					{PathPrefix: "/dev/kmsg"},
				},
				RunAsUser: policyv1beta1.RunAsUserStrategyOptions{
					Rule: policyv1beta1.RunAsUserStrategyRunAsAny,
				},
				SELinux: policyv1beta1.SELinuxStrategyOptions{
					Rule: policyv1beta1.SELinuxStrategyRunAsAny,
				},
				SupplementalGroups: policyv1beta1.SupplementalGroupsStrategyOptions{
					Rule: policyv1beta1.SupplementalGroupsStrategyRunAsAny,
				},
				FSGroup: policyv1beta1.FSGroupStrategyOptions{
					Rule: policyv1beta1.FSGroupStrategyRunAsAny,
				},
			},
		}

		clusterRolePSP = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterRolePSPName,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"extensions", "policy"},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
					ResourceNames: []string{podSecurityPolicy.Name},
				},
			},
		}

		clusterRoleBindingPSP = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        clusterRoleBindingPSPName,
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
				Labels:      getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRolePSP.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
	}

	if version.ConstraintK8sGreaterEqual119.Check(c.values.KubernetesVersion) {
		if daemonSet.Spec.Template.Spec.SecurityContext == nil {
			daemonSet.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}
		daemonSet.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	if c.values.VPAEnabled {
		updateMode := vpaautoscalingv1.UpdateModeAuto
		controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vpaName,
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &v1.CrossVersionObjectReference{
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
								corev1.ResourceCPU:    resource.MustParse("10m"),
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
		clusterRolePSP,
		clusterRoleBindingPSP,
		podSecurityPolicy,
		daemonSet,
		vpa,
	)
}

func (c *nodeProblemDetector) getResourceLimits() corev1.ResourceList {
	if c.values.VPAEnabled {
		return corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("120Mi"),
		}
	}

	return corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("500Mi"),
	}
}

func getLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     labelValue,
		"app.kubernetes.io/instance": "shoot-core",
	}
}
