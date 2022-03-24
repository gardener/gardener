// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	updater                  = "vpa-updater"
	updaterPortServer  int32 = 8080
	updaterPortMetrics int32 = 8943
)

// ValuesUpdater is a set of configuration values for the vpa-updater.
type ValuesUpdater struct {
	// EvictAfterOOMThreshold defines the threshold that will lead to pod eviction in case it OOMed in less than the given
	// threshold since its start and if it has only one container.
	EvictAfterOOMThreshold *metav1.Duration
	// EvictionRateBurst defines the burst of pods that can be evicted.
	EvictionRateBurst *int32
	// EvictionRateLimit defines the number of pods that can be evicted per second. A rate limit set to 0 or -1 will
	// disable the rate limiter.
	EvictionRateLimit *float64
	// EvictionTolerance defines the fraction of replica count that can be evicted for update in case more than one
	// pod can be evicted.
	EvictionTolerance *float64
	// Image is the container image.
	Image string
	// Interval is the interval how often the updater should run.
	Interval *metav1.Duration
	// Replicas is the number of pod replicas.
	Replicas int32
}

func (v *vpa) updaterResourceConfigs() resourceConfigs {
	var (
		clusterRole        = v.emptyClusterRole("evictioner")
		clusterRoleBinding = v.emptyClusterRoleBinding("evictioner")
		deployment         = v.emptyDeployment(updater)
	)

	configs := resourceConfigs{
		{obj: clusterRole, class: application, mutateFn: func() { v.reconcileUpdaterClusterRole(clusterRole) }},
		{obj: clusterRoleBinding, class: application, mutateFn: func() { v.reconcileUpdaterClusterRoleBinding(clusterRoleBinding, clusterRole, updater) }},
	}

	if v.values.ClusterType == ClusterTypeSeed {
		serviceAccount := v.emptyServiceAccount(updater)
		configs = append(configs,
			resourceConfig{obj: serviceAccount, class: application, mutateFn: func() { v.reconcileUpdaterServiceAccount(serviceAccount) }},
			resourceConfig{obj: deployment, class: runtime, mutateFn: func() { v.reconcileUpdaterDeployment(deployment, &serviceAccount.Name) }},
		)
	} else {
		configs = append(configs,
			resourceConfig{obj: deployment, class: runtime, mutateFn: func() { v.reconcileUpdaterDeployment(deployment, nil) }},
		)
	}

	return configs
}

func (v *vpa) reconcileUpdaterServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = getRoleLabel()
	serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
}

func (v *vpa) reconcileUpdaterClusterRole(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{"replicasets"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods/eviction"},
			Verbs:     []string{"create"},
		},
	}
}

func (v *vpa) reconcileUpdaterClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole, serviceAccountName string) {
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

func (v *vpa) reconcileUpdaterDeployment(deployment *appsv1.Deployment, serviceAccountName *string) {
	deployment.Labels = v.getDeploymentLabels(updater)
	deployment.Spec = appsv1.DeploymentSpec{
		Replicas:             &v.values.Updater.Replicas,
		RevisionHistoryLimit: pointer.Int32(2),
		Selector:             &metav1.LabelSelector{MatchLabels: getAppLabel(updater)},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getAllLabels(updater), map[string]string{
					v1beta1constants.LabelNetworkPolicyFromPrometheus: v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            "updater",
					Image:           v.values.Updater.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"./updater"},
					Args: []string{
						"--min-replicas=1",
						fmt.Sprintf("--eviction-tolerance=%f", pointer.Float64Deref(v.values.Updater.EvictionTolerance, gardencorev1beta1.DefaultEvictionTolerance)),
						fmt.Sprintf("--eviction-rate-burst=%d", pointer.Int32Deref(v.values.Updater.EvictionRateBurst, gardencorev1beta1.DefaultEvictionRateBurst)),
						fmt.Sprintf("--eviction-rate-limit=%f", pointer.Float64Deref(v.values.Updater.EvictionRateLimit, gardencorev1beta1.DefaultEvictionRateLimit)),
						fmt.Sprintf("--evict-after-oom-threshold=%s", durationDeref(v.values.Updater.EvictAfterOOMThreshold, gardencorev1beta1.DefaultEvictAfterOOMThreshold).Duration),
						fmt.Sprintf("--updater-interval=%s", durationDeref(v.values.Updater.Interval, gardencorev1beta1.DefaultUpdaterInterval).Duration),
						"--stderrthreshold=info",
						"--v=2",
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "server",
							ContainerPort: updaterPortServer,
						},
						{
							Name:          "metrics",
							ContainerPort: updaterPortMetrics,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("30m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				}},
			},
		},
	}

	if v.values.ClusterType == ClusterTypeShoot {
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	if serviceAccountName != nil {
		deployment.Spec.Template.Spec.ServiceAccountName = *serviceAccountName
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		})
	} else {
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = pointer.Bool(false)
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "KUBERNETES_SERVICE_HOST",
				Value: v1beta1constants.DeploymentNameKubeAPIServer,
			},
			corev1.EnvVar{
				Name:  "KUBERNETES_SERVICE_PORT",
				Value: strconv.Itoa(kubeapiserver.Port),
			},
		)
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "shoot-access",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: pointer.Int32(420),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: v1beta1constants.SecretNameCACluster, // TODO(rfranzke): Use secrets manager for this.
								},
								Items: []corev1.KeyToPath{{
									Key:  secretutils.DataKeyCertificateCA,
									Path: "ca.crt",
								}},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: gutil.SecretNamePrefixShootAccess + updater,
								},
								Items: []corev1.KeyToPath{{
									Key:  resourcesv1alpha1.DataKeyToken,
									Path: "token",
								}},
								Optional: pointer.Bool(false),
							},
						},
					},
				},
			},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "shoot-access",
			MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
			ReadOnly:  true,
		})
	}
}

func durationDeref(ptr *metav1.Duration, def metav1.Duration) metav1.Duration {
	if ptr != nil {
		return *ptr
	}
	return def
}
