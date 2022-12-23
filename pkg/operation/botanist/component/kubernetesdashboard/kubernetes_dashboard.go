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

package kubernetesdashboard

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"

	"github.com/Masterminds/semver"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	name        = "kubernetes-dashboard"
	scraperName = "dashboard-metrics-scraper"
	namespace   = "kubernetes-dashboard"
	labelKey    = "k8s-app"
	labelValue  = "kubernetes-dashboard"
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = name
)

// Values is a set of configuration values for the kubernetes-dashboard component.
type Values struct {
	// APIServerHost is the host of the kube-apiserver.
	APIServerHost *string
	// Image is the container image used for kubernetes-dashboard.
	Image string
	// MetricsScraperImage is the container image used for kubernetes-dashboard metrics scraper.
	MetricsScraperImage string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// AuthenticationMode defines the authentication mode for the kubernetes-dashboard.
	AuthenticationMode string
	// KubernetesVersion is the Kubernetes version of the Shoot.
	KubernetesVersion *semver.Version
}

// New creates a new instance of DeployWaiter for the kubernetes-dashboard.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &kubernetesDashboard{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type kubernetesDashboard struct {
	client    client.Client
	namespace string
	values    Values
}

func (k *kubernetesDashboard) Deploy(ctx context.Context) error {
	data, err := k.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForShoot(ctx, k.client, k.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
}

func (k *kubernetesDashboard) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, k.client, k.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (k *kubernetesDashboard) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, k.client, k.namespace, ManagedResourceName)
}

func (k *kubernetesDashboard) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, k.client, k.namespace, ManagedResourceName)
}

func (k *kubernetesDashboard) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		maxSurge         = intstr.FromInt(0)
		maxUnavailable   = intstr.FromInt(1)
		updateMode       = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly

		namespaces = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					v1beta1constants.GardenerPurpose: name,
				},
			},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels2(name),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"secrets"},
					ResourceNames: []string{"kubernetes-dashboard-key-holder", "kubernetes-dashboard-certs", "kubernetes-dashboard-csrf"},
					Verbs:         []string{"get", "update", "delete"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"services"},
					ResourceNames: []string{"heapster", "dashboard-metrics-scraper"},
					Verbs:         []string{"proxy"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"kubernetes-dashboard-settings"},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"services/proxy"},
					ResourceNames: []string{"heapster", "http:heapster:", "https:heapster:", "dashboard-metrics-scraper", "http:dashboard-metrics-scraper"},
					Verbs:         []string{"get"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels2(name),
				Annotations: map[string]string{
					resourcesv1alpha1.DeleteOnInvalidUpdate: "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      name,
					Namespace: namespace,
				},
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: getLabels2(name),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"metrics.k8s.io"},
					Resources: []string{"pods", "nodes"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					resourcesv1alpha1.DeleteOnInvalidUpdate: "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      name,
					Namespace: namespace,
				},
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels2(name),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		secret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-certs",
				Namespace: namespace,
				Labels:    getLabels2(name),
			},
			Type: corev1.SecretTypeOpaque,
		}

		secret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-csrf",
				Namespace: namespace,
				Labels:    getLabels2(name),
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"csrf": {},
			},
		}

		secret3 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-key-holder",
				Namespace: namespace,
				Labels:    getLabels2(name),
			},
			Type: corev1.SecretTypeOpaque,
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-settings",
				Namespace: namespace,
				Labels:    getLabels2(name),
			},
		}

		deploymentDashboard = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels(name),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(1),
				Replicas:             pointer.Int32(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels2(name),
				},
				Strategy: appsv1.DeploymentStrategy{
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       &maxSurge,
						MaxUnavailable: &maxUnavailable,
					},
					Type: appsv1.RollingUpdateDeploymentStrategyType,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(name),
					},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{
							v1beta1constants.LabelWorkerPoolSystemComponents: "true",
						},
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:          pointer.Int64(1001),
							RunAsGroup:         pointer.Int64(2001),
							FSGroup:            pointer.Int64(1),
							SupplementalGroups: []int64{1},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            name,
								Image:           k.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--auto-generate-certificates",
									"--authentication-mode=" + k.values.AuthenticationMode,
									"--namespace=" + namespace,
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: int32(8443),
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Scheme: corev1.URISchemeHTTPS,
											Path:   "/",
											Port:   intstr.FromInt(8443),
										},
									},
									InitialDelaySeconds: int32(30),
									TimeoutSeconds:      int32(30),
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: pointer.Bool(false),
									ReadOnlyRootFilesystem:   pointer.Bool(true),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "kubernetes-dashboard-certs",
										MountPath: "/certs",
									},
									{
										Name:      "tmp-volume",
										MountPath: "/tmp",
									},
								},
							},
						},
						ServiceAccountName: name,
						Volumes: []corev1.Volume{
							{
								Name: "kubernetes-dashboard-certs",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "kubernetes-dashboard-certs",
									},
								},
							},
							{
								Name: "tmp-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
			},
		}

		deploymentMetricsScraper = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scraperName,
				Namespace: namespace,
				Labels:    getLabels(scraperName),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(1),
				Replicas:             pointer.Int32(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels2(scraperName),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(scraperName),
					},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{
							v1beta1constants.LabelWorkerPoolSystemComponents: "true",
						},
						SecurityContext: &corev1.PodSecurityContext{
							FSGroup:            pointer.Int64(1),
							SupplementalGroups: []int64{1},
						},
						Containers: []corev1.Container{
							{
								Name:  scraperName,
								Image: k.values.MetricsScraperImage,
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: int32(8000),
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Scheme: corev1.URISchemeHTTP,
											Path:   "/",
											Port:   intstr.FromInt(8000),
										},
									},
									InitialDelaySeconds: int32(30),
									TimeoutSeconds:      int32(30),
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: pointer.Bool(false),
									ReadOnlyRootFilesystem:   pointer.Bool(true),
									RunAsUser:                pointer.Int64(1001),
									RunAsGroup:               pointer.Int64(2001),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "tmp-volume",
										MountPath: "/tmp",
									},
								},
							},
						},
						ServiceAccountName: name,
						Volumes: []corev1.Volume{
							{
								Name: "tmp-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
			},
		}

		serviceDashboard = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels2(name),
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       int32(443),
						TargetPort: intstr.FromInt(8443),
					},
				},
				Selector: getLabels2(name),
			},
		}

		serviceMetricsScraper = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scraperName,
				Namespace: namespace,
				Labels:    getLabels2(scraperName),
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       int32(8000),
						TargetPort: intstr.FromInt(8000),
					},
				},
				Selector: getLabels2(scraperName),
			},
		}

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	if k.values.VPAEnabled {
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &v1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deploymentDashboard.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &updateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "*",
							ControlledValues: &controlledValues,
						},
					},
				},
			},
		}
	}

	if version.ConstraintK8sLessEqual122.Check(k.values.KubernetesVersion) {
		deploymentMetricsScraper.Annotations = map[string]string{corev1.SeccompPodAnnotationKey: corev1.SeccompProfileRuntimeDefault}
	}

	if version.ConstraintK8sGreaterEqual123.Check(k.values.KubernetesVersion) {
		deploymentMetricsScraper.Spec.Template.Spec.Containers[0].SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	if k.values.APIServerHost != nil {
		deploymentDashboard.Spec.Template.Spec.Containers[0].Env = append(deploymentDashboard.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_HOST",
			Value: *k.values.APIServerHost,
		})
	}

	return registry.AddAllAndSerialize(
		namespaces,
		role,
		roleBinding,
		clusterRole,
		clusterRoleBinding,
		serviceAccount,
		secret1,
		secret2,
		secret3,
		configMap,
		deploymentDashboard,
		deploymentMetricsScraper,
		serviceDashboard,
		serviceMetricsScraper,
		vpa,
	)
}

func getLabels(labelValue string) map[string]string {
	return map[string]string{
		"origin":                    "gardener",
		labelKey:                    labelValue,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleOptionalAddon,
	}
}

func getLabels2(labelValue string) map[string]string {
	return map[string]string{
		labelKey: labelValue,
	}
}
