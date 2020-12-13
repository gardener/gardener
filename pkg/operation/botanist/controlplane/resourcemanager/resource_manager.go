// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package resourcemanager

import (
	"context"
	"errors"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// SecretName is a constant for the secret name for the gardener resource manager's kubeconfig secret.
	SecretName = "gardener-resource-manager"
	// UserName is the name that should be used for the secret that the gardener resource manager uses to
	// authenticate itself with the kube-apiserver (e.g., the common name in its client certificate).
	UserName = "gardener-resource-manager"

	containerName      = v1beta1constants.DeploymentNameGardenerResourceManager
	healthPort         = 8081
	metricsPort        = 8080
	roleName           = "gardener-resource-manager"
	serviceAccountName = "gardener-resource-manager"
)

var (
	allowAll = []rbacv1.PolicyRule{{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	}}

	allowManagedResources = []rbacv1.PolicyRule{{
		APIGroups: []string{"resources.gardener.cloud"},
		Resources: []string{"managedresources", "managedresources/status"},
		Verbs:     []string{"get", "list", "watch", "update", "patch"},
	}, {
		APIGroups: []string{""},
		Resources: []string{"secrets"},
		Verbs:     []string{"get", "list", "watch", "update", "patch"},
	}, {
		APIGroups: []string{""},
		Resources: []string{"configmaps", "events"},
		Verbs:     []string{"create"},
	}, {
		APIGroups:     []string{""},
		Resources:     []string{"configmaps"},
		ResourceNames: []string{"gardener-resource-manager"},
		Verbs:         []string{"get", "watch", "update", "patch"},
	}}
)

// New creates a new instance of the gardener-resource-manager.
func New(
	client client.Client,
	namespace string,
	image string,
	replicas int32,
	config Config,
) *ResourceManager {
	if config.DefaultLabels == nil {
		config.DefaultLabels = appLabel()
	} else {
		defaultLabelsWithAppLabel := utils.MergeStringMaps(*config.DefaultLabels, *appLabel())
		config.DefaultLabels = &defaultLabelsWithAppLabel
	}
	return &ResourceManager{
		client:    client,
		image:     image,
		namespace: namespace,
		replicas:  replicas,
		config:    config,
	}
}

type ResourceManager struct {
	client    client.Client
	namespace string
	image     string
	replicas  int32
	config    Config
}

// Config holds the optional configuration options for the gardener resource manager
type Config struct {
	AlwaysUpdate               *bool
	ConcurrentSyncs            *int32
	ClusterRoleName            *string
	DefaultLabels              *map[string]string
	HealthSyncPeriod           *string
	KubeConfig                 *component.Secret
	LeaseDuration              *string
	MaxConcurrentHealthWorkers *int32
	RenewDeadline              *string
	ResourceClass              *string
	RetryPeriod                *string
	SyncPeriod                 *string
	TargetDisableCache         *bool
	WatchedNamespace           *string
}

func (r *ResourceManager) Deploy(ctx context.Context) error {
	if err := r.ensureServiceAccount(ctx); err != nil {
		return err
	}

	if err := r.ensureRBAC(ctx); err != nil {
		return err
	}

	if err := r.ensureService(ctx); err != nil {
		return err
	}

	if err := r.ensureDeployment(ctx); err != nil {
		return err
	}

	return r.ensureVPA(ctx)
}

func (r *ResourceManager) Destroy(ctx context.Context) error {
	objectsToDelete := []client.Object{
		r.emptyVPA(),
		r.emptyDeployment(),
		r.emptyService(),
		r.emptyServiceAccount(),
	}
	role, err := r.emptyRoleInWatchedNamespace()
	if err == nil {
		objectsToDelete = append(objectsToDelete, role)
	}
	rb, err := r.emptyRoleBindingInWatchedNamespace()
	if err == nil {
		objectsToDelete = append(objectsToDelete, rb)
	}
	cr, err := r.emptyClusterRole()
	if err == nil {
		objectsToDelete = append(objectsToDelete, cr)
	}
	crb, err := r.emptyClusterRoleBinding()
	if err == nil {
		objectsToDelete = append(objectsToDelete, crb)
	}

	return kutil.DeleteObjects(
		ctx,
		r.client,
		objectsToDelete...,
	)
}

func (r *ResourceManager) ensureRBAC(ctx context.Context) error {
	deployResourcesIntoAnotherCluster := r.config.KubeConfig != nil
	if deployResourcesIntoAnotherCluster {
		if r.config.WatchedNamespace == nil {
			if err := r.ensureClusterRole(ctx, allowManagedResources); err != nil {
				return err
			}
			if err := r.ensureClusterRoleBinding(ctx); err != nil {
				return err
			}
		} else {
			if err := r.ensureRoleInWatchedNamespace(ctx, allowManagedResources); err != nil {
				return err
			}
			if err := r.ensureRoleBinding(ctx); err != nil {
				return err
			}
		}
	} else {
		if err := r.ensureClusterRole(ctx, allowAll); err != nil {
			return err
		}
		if err := r.ensureClusterRoleBinding(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResourceManager) ensureClusterRole(ctx context.Context, policies []rbacv1.PolicyRule) error {
	clusterRole, err := r.emptyClusterRole()
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, clusterRole, func() error {
		clusterRole.Labels = *r.config.DefaultLabels
		clusterRole.Rules = policies
		return nil
	})
	return err
}

func (r *ResourceManager) emptyClusterRole() (*rbacv1.ClusterRole, error) {
	if r.config.ClusterRoleName == nil {
		return nil, errors.New("creating Cluster Role failed - no name defined for the Cluster Role")
	}
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: *r.config.ClusterRoleName}}, nil
}

func (r *ResourceManager) ensureClusterRoleBinding(ctx context.Context) error {
	clusterRoleBinding, err := r.emptyClusterRoleBinding()
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, clusterRoleBinding, func() error {
		clusterRoleBinding.Labels = *r.config.DefaultLabels
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     *r.config.ClusterRoleName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      UserName,
			Namespace: r.namespace,
		}}
		return nil
	})
	return err
}

func (r *ResourceManager) emptyClusterRoleBinding() (*rbacv1.ClusterRoleBinding, error) {
	if r.config.ClusterRoleName == nil {
		return nil, errors.New("creating Cluster Role Binding failed - no name defined for the Cluster Role")
	}
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: *r.config.ClusterRoleName}}, nil
}

func (r *ResourceManager) ensureRoleInWatchedNamespace(ctx context.Context, policies []rbacv1.PolicyRule) error {
	role, err := r.emptyRoleInWatchedNamespace()
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, role, func() error {
		role.Labels = *r.config.DefaultLabels
		role.Rules = policies
		return nil
	})
	return err
}

func (r *ResourceManager) emptyRoleInWatchedNamespace() (*rbacv1.Role, error) {
	if r.config.WatchedNamespace == nil {
		return nil, errors.New("creating Role in watched namespace failed - no namespace defined")
	}
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: *r.config.WatchedNamespace}}, nil
}

func (r *ResourceManager) ensureRoleBinding(ctx context.Context) error {
	roleBinding, err := r.emptyRoleBindingInWatchedNamespace()
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, roleBinding, func() error {
		roleBinding.Labels = *r.config.DefaultLabels
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		}
		roleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: r.namespace,
		}}
		return nil
	})
	return err
}

func (r *ResourceManager) emptyRoleBindingInWatchedNamespace() (*rbacv1.RoleBinding, error) {
	if r.config.WatchedNamespace == nil {
		return nil, errors.New("creating RoleBinding in watched namespace failed - no namespace defined")
	}
	return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: *r.config.WatchedNamespace}}, nil
}

func (r *ResourceManager) ensureService(ctx context.Context) error {
	const (
		healthPortName  = "health"
		metricsPortName = "metrics"
	)

	service := r.emptyService()
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, service, func() error {
		service.Labels = *r.config.DefaultLabels
		service.Spec.Selector = *appLabel()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, []corev1.ServicePort{
			{
				Name:     metricsPortName,
				Protocol: corev1.ProtocolTCP,
				Port:     metricsPort,
			},
			{
				Name:     healthPortName,
				Protocol: corev1.ProtocolTCP,
				Port:     healthPort,
			},
		})
		return nil
	})
	return err
}

func (r *ResourceManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: r.namespace}}
}

func (r *ResourceManager) ensureDeployment(ctx context.Context) error {
	const (
		limitCPU                  = "400m"
		limitMemory               = "512Mi"
		requestCPU                = "23m"
		requestMemory             = "47Mi"
		volumeMountName           = "gardener-resource-manager"
		volumeMountPathKubeconfig = "/etc/gardener-resource-manager"
		volumeName                = "gardener-resource-manager"
	)

	deployment := r.emptyDeployment()

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, deployment, func() error {
		deployment.Labels = *r.config.DefaultLabels

		deployment.Spec.Replicas = &r.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32Ptr(0)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: *appLabel()}

		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(*r.config.DefaultLabels, map[string]string{
					v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToSeedAPIServer:  v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           r.image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         r.printCommand(volumeMountPathKubeconfig),
						Ports: []corev1.ContainerPort{
							{
								Name:          "metrics",
								ContainerPort: metricsPort,
								Protocol:      corev1.ProtocolTCP,
							},
							{
								Name:          "health",
								ContainerPort: healthPort,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(requestCPU),
								corev1.ResourceMemory: resource.MustParse(requestMemory),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(limitCPU),
								corev1.ResourceMemory: resource.MustParse(limitMemory),
							},
						},
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Scheme: "HTTP",
									Port:   intstr.FromInt(healthPort),
								},
							},
							InitialDelaySeconds: 30,
							FailureThreshold:    5,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      5,
						},
					},
				},
			},
		}
		if r.config.KubeConfig != nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{
				"checksum/secret-" + r.config.KubeConfig.Name: r.config.KubeConfig.Checksum,
			}
			deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  r.config.KubeConfig.Name,
							DefaultMode: pointer.Int32Ptr(420),
						},
					},
				},
			}
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					MountPath: volumeMountPathKubeconfig,
					Name:      volumeMountName,
					ReadOnly:  true,
				},
			}
		}

		// TODO(beckermax) remove in a future version
		// Leave garden.sapcloud.io/role in controlplane pods for compatibility reasons
		if v, ok := deployment.Labels[v1beta1constants.GardenRole]; ok && v == v1beta1constants.GardenRoleControlPlane {
			deployment.Spec.Template.ObjectMeta.Labels = utils.MergeStringMaps(deployment.Spec.Template.ObjectMeta.Labels, map[string]string{
				v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
			})
		}

		return nil
	})
	return err
}

func (r *ResourceManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: r.namespace}}
}

func (r *ResourceManager) printCommand(volumeMountPathKubeconfig string) []string {
	cmd := []string{
		"/gardener-resource-manager",
	}
	if r.config.AlwaysUpdate != nil {
		cmd = append(cmd, fmt.Sprintf("--always-update=%v", *r.config.AlwaysUpdate))
	}
	cmd = append(cmd, fmt.Sprintf("--health-bind-address=:%v", healthPort))
	if r.config.MaxConcurrentHealthWorkers != nil {
		cmd = append(cmd, fmt.Sprintf("--health-max-concurrent-workers=%v", *r.config.MaxConcurrentHealthWorkers))
	}
	if r.config.HealthSyncPeriod != nil {
		cmd = append(cmd, fmt.Sprintf("--health-sync-period=%s", *r.config.HealthSyncPeriod))
	}
	cmd = append(cmd, "--leader-election=true")
	if r.config.LeaseDuration != nil {
		cmd = append(cmd, fmt.Sprintf("--leader-election-lease-duration=%s", *r.config.LeaseDuration))
	}
	cmd = append(cmd, fmt.Sprintf("--leader-election-namespace=%s", r.namespace))
	if r.config.RenewDeadline != nil {
		cmd = append(cmd, fmt.Sprintf("--leader-election-renew-deadline=%s", *r.config.RenewDeadline))
	}
	if r.config.RetryPeriod != nil {
		cmd = append(cmd, fmt.Sprintf("--leader-election-retry-period=%s", *r.config.RetryPeriod))
	}
	if r.config.ConcurrentSyncs != nil {
		cmd = append(cmd, fmt.Sprintf("--max-concurrent-workers=%v", *r.config.ConcurrentSyncs))
	}
	cmd = append(cmd, fmt.Sprintf("--metrics-bind-address=:%v", metricsPort))
	if r.config.WatchedNamespace != nil {
		cmd = append(cmd, fmt.Sprintf("--namespace=%s", *r.config.WatchedNamespace))
	}
	if r.config.ResourceClass != nil {
		cmd = append(cmd, fmt.Sprintf("--resource-class=%s", *r.config.ResourceClass))
	}
	if r.config.SyncPeriod != nil {
		cmd = append(cmd, fmt.Sprintf("--sync-period=%s", *r.config.SyncPeriod))
	}
	if r.config.TargetDisableCache != nil {
		cmd = append(cmd, "--target-disable-cache")
	}
	if r.config.KubeConfig != nil {
		cmd = append(cmd, fmt.Sprintf("--target-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig))
	}
	return cmd
}

func (r *ResourceManager) ensureServiceAccount(ctx context.Context) error {
	serviceAccount := r.emptyServiceAccount()
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, serviceAccount, func() error {
		serviceAccount.Labels = *r.config.DefaultLabels
		return nil
	})
	return err
}

func (r *ResourceManager) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: r.namespace}}
}

func (r *ResourceManager) ensureVPA(ctx context.Context) error {
	vpa := r.emptyVPA()
	vpaUpdateMode := autoscalingv1beta2.UpdateModeAuto

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, vpa, func() error {
		vpa.Labels = *r.config.DefaultLabels
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameGardenerResourceManager,
		}
		vpa.Spec.UpdatePolicy = &autoscalingv1beta2.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		return nil
	})
	return err
}

func (r *ResourceManager) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: r.namespace}}
}

func appLabel() *map[string]string {
	return &map[string]string{
		v1beta1constants.LabelApp: "gardener-resource-manager",
	}
}

// SetKubeConfig enables the ResourceManager to deploy resources into a different
// cluster than the one it is running in.
func (r *ResourceManager) SetKubeConfig(k *component.Secret) { r.config.KubeConfig = k }

func (r *ResourceManager) Wait(_ context.Context) error        { return nil }
func (r *ResourceManager) WaitCleanup(_ context.Context) error { return nil }
