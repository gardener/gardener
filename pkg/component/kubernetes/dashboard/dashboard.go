// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"time"

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
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	name        = "kubernetes-dashboard"
	scraperName = "dashboard-metrics-scraper"
	labelKey    = "k8s-app"
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "shoot-addon-kubernetes-dashboard"
)

// Interface contains functions for a kubernetes-dashboard deployer.
type Interface interface {
	component.DeployWaiter
}

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
}

// New creates a new instance of DeployWaiter for the kubernetes-dashboard.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
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

		maxSurge         = intstr.FromInt32(0)
		maxUnavailable   = intstr.FromInt32(1)
		updateMode       = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1beta1constants.KubernetesDashboardNamespace,
				Labels: map[string]string{
					v1beta1constants.GardenerPurpose: name,
				},
			},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
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
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
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
					Namespace: v1beta1constants.KubernetesDashboardNamespace,
				},
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{labelKey: name},
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
					Namespace: v1beta1constants.KubernetesDashboardNamespace,
				},
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		secretCerts = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-certs",
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
			},
			Type: corev1.SecretTypeOpaque,
		}

		secretCSRF = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-csrf",
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"csrf": {},
			},
		}

		secretKeyHolder = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-key-holder",
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
			},
			Type: corev1.SecretTypeOpaque,
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-settings",
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
			},
		}

		deploymentDashboard = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameKubernetesDashboard,
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    getLabels(name),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: ptr.To[int32](2),
				Replicas:             ptr.To[int32](1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{labelKey: name},
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
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot:       ptr.To(true),
							RunAsUser:          ptr.To[int64](1001),
							RunAsGroup:         ptr.To[int64](2001),
							FSGroup:            ptr.To[int64](1),
							SupplementalGroups: []int64{1},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            v1beta1constants.DeploymentNameKubernetesDashboard,
								Image:           k.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--auto-generate-certificates",
									"--authentication-mode=" + k.values.AuthenticationMode,
									"--namespace=" + v1beta1constants.KubernetesDashboardNamespace,
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
											Port:   intstr.FromInt32(8443),
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
									AllowPrivilegeEscalation: ptr.To(false),
									ReadOnlyRootFilesystem:   ptr.To(true),
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
				Name:      v1beta1constants.DeploymentNameDashboardMetricsScraper,
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    getLabels(scraperName),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: ptr.To[int32](2),
				Replicas:             ptr.To[int32](1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{labelKey: scraperName},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(scraperName),
					},
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{
							FSGroup:            ptr.To[int64](1),
							SupplementalGroups: []int64{1},
						},
						Containers: []corev1.Container{
							{
								Name:  v1beta1constants.DeploymentNameDashboardMetricsScraper,
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
											Port:   intstr.FromInt32(8000),
										},
									},
									InitialDelaySeconds: int32(30),
									TimeoutSeconds:      int32(30),
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									ReadOnlyRootFilesystem:   ptr.To(true),
									RunAsNonRoot:             ptr.To(true),
									RunAsUser:                ptr.To[int64](1001),
									RunAsGroup:               ptr.To[int64](2001),
									SeccompProfile: &corev1.SeccompProfile{
										Type: corev1.SeccompProfileTypeRuntimeDefault,
									},
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
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: name},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       int32(443),
						TargetPort: intstr.FromInt32(8443),
					},
				},
				Selector: map[string]string{labelKey: name},
			},
		}

		serviceMetricsScraper = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scraperName,
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
				Labels:    map[string]string{labelKey: scraperName},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       int32(8000),
						TargetPort: intstr.FromInt32(8000),
					},
				},
				Selector: map[string]string{labelKey: scraperName},
			},
		}

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	if k.values.VPAEnabled {
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: v1beta1constants.KubernetesDashboardNamespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
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

	if k.values.APIServerHost != nil {
		deploymentDashboard.Spec.Template.Spec.Containers[0].Env = append(deploymentDashboard.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_HOST",
			Value: *k.values.APIServerHost,
		})
	}

	return registry.AddAllAndSerialize(
		namespace,
		role,
		roleBinding,
		clusterRole,
		clusterRoleBinding,
		serviceAccount,
		secretCerts,
		secretCSRF,
		secretKeyHolder,
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
