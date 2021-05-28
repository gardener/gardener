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

package resourcemanager

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	UserName = "gardener.cloud:system:gardener-resource-manager"

	clusterRoleName           = "gardener-resource-manager-seed"
	containerName             = v1beta1constants.DeploymentNameGardenerResourceManager
	healthPort                = 8081
	metricsPort               = 8080
	roleName                  = "gardener-resource-manager"
	serviceAccountName        = "gardener-resource-manager"
	volumeMountPathKubeconfig = "/etc/gardener-resource-manager"
)

var (
	allowAll = []rbacv1.PolicyRule{{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	}}

	allowManagedResources = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"resources.gardener.cloud"},
			Resources: []string{"managedresources", "managedresources/status"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps", "events"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{"gardener-resource-manager"},
			Verbs:         []string{"get", "watch", "update", "patch"},
		},
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups:     []string{"coordination.k8s.io"},
			Resources:     []string{"leases"},
			ResourceNames: []string{"gardener-resource-manager"},
			Verbs:         []string{"get", "watch", "update"},
		},
	}
)

// Interface contains functions for a gardener-resource-manager deployer.
type Interface interface {
	component.DeployWaiter
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
}

// New creates a new instance of the gardener-resource-manager.
func New(
	client client.Client,
	namespace string,
	image string,
	replicas int32,
	values Values,
) Interface {
	return &resourceManager{
		client:    client,
		image:     image,
		namespace: namespace,
		replicas:  replicas,
		values:    values,
	}
}

type resourceManager struct {
	client    client.Client
	namespace string
	image     string
	replicas  int32
	values    Values
}

// Values holds the optional configuration options for the gardener resource manager
type Values struct {
	// AlwaysUpdate if set to false then a resource will only be updated if its desired state differs from the actual state. otherwise, an update request will be always sent
	AlwaysUpdate *bool
	// ClusterIdentity is the identity of the managing cluster.
	ClusterIdentity *string
	// ConcurrentSyncs are the number of worker threads for concurrent reconciliation of resources
	ConcurrentSyncs *int32
	// HealthSyncPeriod describes the duration of how often the health of existing resources should be synced
	HealthSyncPeriod *time.Duration
	// Kubeconfig configures the gardener-resource-manager to target another cluster for creating resources.
	// If this is not set resources are created in the cluster the gardener-resource-manager is deployed in
	Kubeconfig *component.Secret
	// LeaseDuration configures the lease duration for leader election
	LeaseDuration *time.Duration
	// MaxConcurrentHealthWorkers configures the number of worker threads for concurrent health reconciliation of resources
	MaxConcurrentHealthWorkers *int32
	// RenewDeadline configures the renew deadline for leader election
	RenewDeadline *time.Duration
	// ResourceClass is used to filter resource resources
	ResourceClass *string
	// RetryPeriod configures the retry period for leader election
	RetryPeriod *time.Duration
	// SyncPeriod configures the duration of how often existing resources should be synced
	SyncPeriod *time.Duration
	// TargetDisableCache disables the cache for target cluster and always talk directly to the API server (defaults to false)
	TargetDisableCache *bool
	// WatchedNamespace restricts the gardener-resource-manager to only watch ManagedResources in the defined namespace.
	// If not set the gardener-resource-manager controller watches for ManagedResources in all namespaces
	WatchedNamespace *string
}

func (r *resourceManager) Deploy(ctx context.Context) error {
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

func (r *resourceManager) Destroy(ctx context.Context) error {
	objectsToDelete := []client.Object{
		r.emptyVPA(),
		r.emptyDeployment(),
		r.emptyService(),
		r.emptyServiceAccount(),
		r.emptyClusterRole(),
		r.emptyClusterRoleBinding(),
	}
	role, err := r.emptyRoleInWatchedNamespace()
	if err == nil {
		objectsToDelete = append(objectsToDelete, role)
	}
	rb, err := r.emptyRoleBindingInWatchedNamespace()
	if err == nil {
		objectsToDelete = append(objectsToDelete, rb)
	}

	return kutil.DeleteObjects(
		ctx,
		r.client,
		objectsToDelete...,
	)
}

func (r *resourceManager) ensureRBAC(ctx context.Context) error {
	targetDiffersFromSourceCluster := r.values.Kubeconfig != nil
	if targetDiffersFromSourceCluster {
		if r.values.WatchedNamespace == nil {
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

func (r *resourceManager) ensureClusterRole(ctx context.Context, policies []rbacv1.PolicyRule) error {
	clusterRole := r.emptyClusterRole()
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, clusterRole, func() error {
		clusterRole.Labels = r.getLabels()
		clusterRole.Rules = policies
		return nil
	})
	return err
}

func (r *resourceManager) emptyClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}
}

func (r *resourceManager) ensureClusterRoleBinding(ctx context.Context) error {
	clusterRoleBinding := r.emptyClusterRoleBinding()
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, clusterRoleBinding, func() error {
		clusterRoleBinding.Labels = r.getLabels()
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: r.namespace,
		}}
		return nil
	})
	return err
}

func (r *resourceManager) emptyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}
}

func (r *resourceManager) ensureRoleInWatchedNamespace(ctx context.Context, policies []rbacv1.PolicyRule) error {
	role, err := r.emptyRoleInWatchedNamespace()
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, role, func() error {
		role.Labels = r.getLabels()
		role.Rules = policies
		return nil
	})
	return err
}

func (r *resourceManager) emptyRoleInWatchedNamespace() (*rbacv1.Role, error) {
	if r.values.WatchedNamespace == nil {
		return nil, errors.New("creating Role in watched namespace failed - no namespace defined")
	}
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: *r.values.WatchedNamespace}}, nil
}

func (r *resourceManager) ensureRoleBinding(ctx context.Context) error {
	roleBinding, err := r.emptyRoleBindingInWatchedNamespace()
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, roleBinding, func() error {
		roleBinding.Labels = r.getLabels()
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

func (r *resourceManager) emptyRoleBindingInWatchedNamespace() (*rbacv1.RoleBinding, error) {
	if r.values.WatchedNamespace == nil {
		return nil, errors.New("creating RoleBinding in watched namespace failed - no namespace defined")
	}
	return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: *r.values.WatchedNamespace}}, nil
}

func (r *resourceManager) ensureService(ctx context.Context) error {
	const (
		healthPortName  = "health"
		metricsPortName = "metrics"
	)

	service := r.emptyService()
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, service, func() error {
		service.Labels = r.getLabels()
		service.Spec.Selector = appLabel()
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

func (r *resourceManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: r.namespace}}
}

func (r *resourceManager) ensureDeployment(ctx context.Context) error {
	deployment := r.emptyDeployment()

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, deployment, func() error {
		deployment.Labels = r.getLabels()

		deployment.Spec.Replicas = &r.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32Ptr(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabel()}

		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: r.getDeploymentTemplateLabels(),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           r.image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         r.computeCommand(),
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
								corev1.ResourceCPU:    resource.MustParse("23m"),
								corev1.ResourceMemory: resource.MustParse("47Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("400m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
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

		if r.values.Kubeconfig != nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{
				"checksum/secret-" + r.values.Kubeconfig.Name: r.values.Kubeconfig.Checksum,
			}
			deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "gardener-resource-manager",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  r.values.Kubeconfig.Name,
							DefaultMode: pointer.Int32Ptr(420),
						},
					},
				},
			}
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					MountPath: volumeMountPathKubeconfig,
					Name:      "gardener-resource-manager",
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

func (r *resourceManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: r.namespace}}
}

func (r *resourceManager) computeCommand() []string {
	cmd := []string{
		"/gardener-resource-manager",
	}
	if r.values.AlwaysUpdate != nil {
		cmd = append(cmd, fmt.Sprintf("--always-update=%v", *r.values.AlwaysUpdate))
	}
	if r.values.ClusterIdentity != nil {
		cmd = append(cmd, fmt.Sprintf("--cluster-id=%s", *r.values.ClusterIdentity))
	}
	cmd = append(cmd, fmt.Sprintf("--health-bind-address=:%v", healthPort))
	if r.values.MaxConcurrentHealthWorkers != nil {
		cmd = append(cmd, fmt.Sprintf("--health-max-concurrent-workers=%d", *r.values.MaxConcurrentHealthWorkers))
	}
	if r.values.HealthSyncPeriod != nil {
		cmd = append(cmd, fmt.Sprintf("--health-sync-period=%s", *r.values.HealthSyncPeriod))
	}
	cmd = append(cmd, "--leader-election=true")
	if r.values.LeaseDuration != nil {
		cmd = append(cmd, fmt.Sprintf("--leader-election-lease-duration=%s", *r.values.LeaseDuration))
	}
	cmd = append(cmd, fmt.Sprintf("--leader-election-namespace=%s", r.namespace))
	if r.values.RenewDeadline != nil {
		cmd = append(cmd, fmt.Sprintf("--leader-election-renew-deadline=%s", *r.values.RenewDeadline))
	}
	if r.values.RetryPeriod != nil {
		cmd = append(cmd, fmt.Sprintf("--leader-election-retry-period=%s", *r.values.RetryPeriod))
	}
	if r.values.ConcurrentSyncs != nil {
		cmd = append(cmd, fmt.Sprintf("--max-concurrent-workers=%d", *r.values.ConcurrentSyncs))
	}
	cmd = append(cmd, fmt.Sprintf("--metrics-bind-address=:%d", metricsPort))
	if r.values.WatchedNamespace != nil {
		cmd = append(cmd, fmt.Sprintf("--namespace=%s", *r.values.WatchedNamespace))
	}
	if r.values.ResourceClass != nil {
		cmd = append(cmd, fmt.Sprintf("--resource-class=%s", *r.values.ResourceClass))
	}
	if r.values.SyncPeriod != nil {
		cmd = append(cmd, fmt.Sprintf("--sync-period=%s", *r.values.SyncPeriod))
	}
	if r.values.TargetDisableCache != nil {
		cmd = append(cmd, "--target-disable-cache")
	}
	if r.values.Kubeconfig != nil {
		cmd = append(cmd, fmt.Sprintf("--target-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig))
	}
	return cmd
}

func (r *resourceManager) ensureServiceAccount(ctx context.Context) error {
	serviceAccount := r.emptyServiceAccount()
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, serviceAccount, func() error {
		serviceAccount.Labels = r.getLabels()
		return nil
	})
	return err
}

func (r *resourceManager) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: r.namespace}}
}

func (r *resourceManager) ensureVPA(ctx context.Context) error {
	vpa := r.emptyVPA()
	vpaUpdateMode := autoscalingv1beta2.UpdateModeAuto

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, vpa, func() error {
		vpa.Labels = r.getLabels()
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

func (r *resourceManager) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager-vpa", Namespace: r.namespace}}
}

func (r *resourceManager) getLabels() map[string]string {
	partOfShootControlPlane := r.values.Kubeconfig != nil
	if partOfShootControlPlane {
		return utils.MergeStringMaps(appLabel(), map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		})
	}

	return appLabel()
}

func (r *resourceManager) getDeploymentTemplateLabels() map[string]string {
	partOfShootControlPlane := r.values.Kubeconfig != nil
	if partOfShootControlPlane {
		return utils.MergeStringMaps(appLabel(), map[string]string{
			v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToSeedAPIServer:  v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	return appLabel()
}

func appLabel() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: "gardener-resource-manager",
	}
}

// Wait signals whether a deployment is ready or needs more time to be deployed. Gardener-Resource-Manager is ready immediately.
func (r *resourceManager) Wait(_ context.Context) error { return nil }

// WaitCleanup for destruction to finish and component to be fully removed. Gardener-Resource-manager does not need to wait for cleanup.
func (r *resourceManager) WaitCleanup(_ context.Context) error { return nil }

// SetSecrets sets the secrets for the gardener-resource-manager.
func (r *resourceManager) SetSecrets(s Secrets) { r.values.Kubeconfig = &s.Kubeconfig }

// Secrets is collection of secrets for the gardener-resource-manager.
type Secrets struct {
	// Kubeconfig enables the gardener-resource-manager to deploy resources into a different cluster than the one it is running in.
	Kubeconfig component.Secret
}
