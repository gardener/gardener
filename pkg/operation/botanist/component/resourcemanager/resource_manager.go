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
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/podschedulername"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/projectedtokenmount"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/seccompprofile"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/tokeninvalidator"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	"github.com/Masterminds/semver"
	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceName is the name for the ManagedResource containing resources deployed to the shoot cluster.
	ManagedResourceName = "shoot-core-gardener-resource-manager"
	// SecretNameShootAccess is the name of the shoot access secret for the gardener-resource-manager.
	SecretNameShootAccess = gutil.SecretNamePrefixShootAccess + v1beta1constants.DeploymentNameGardenerResourceManager
	// LabelValue is a constant for the value of the 'app' label on Kubernetes resources.
	LabelValue = "gardener-resource-manager"

	serviceName        = "gardener-resource-manager"
	secretNameServer   = "gardener-resource-manager-server"
	clusterRoleName    = "gardener-resource-manager-seed"
	roleName           = "gardener-resource-manager"
	serviceAccountName = "gardener-resource-manager"
	metricsPortName    = "metrics"
	containerName      = v1beta1constants.DeploymentNameGardenerResourceManager
	healthPort         = 8081
	metricsPort        = 8080
	serverPort         = 10250
	serverServicePort  = 443

	volumeNameBootstrapKubeconfig  = "kubeconfig-bootstrap"
	volumeNameCerts                = "tls"
	volumeNameAPIServerAccess      = "kube-api-access-gardener"
	volumeMountPathCerts           = "/etc/gardener-resource-manager-tls"
	volumeMountPathAPIServerAccess = "/var/run/secrets/kubernetes.io/serviceaccount"

	volumeNameRootCA      = "root-ca"
	volumeMountPathRootCA = "/etc/gardener-resource-manager-root-ca"
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
	component.MonitoringComponent
	// GetReplicas gets the Replicas field in the Values.
	GetReplicas() *int32
	// SetReplicas sets the Replicas field in the Values.
	SetReplicas(*int32)
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
}

// New creates a new instance of the gardener-resource-manager.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	image string,
	values Values,
) Interface {
	return &resourceManager{
		client:         client,
		image:          image,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type resourceManager struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	image          string
	values         Values
	secrets        Secrets
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
	// LeaseDuration configures the lease duration for leader election
	LeaseDuration *time.Duration
	// MaxConcurrentHealthWorkers configures the number of worker threads for concurrent health reconciliation of resources
	MaxConcurrentHealthWorkers *int32
	// MaxConcurrentTokenInvalidatorWorkers configures the number of worker threads for concurrent token invalidator reconciliations
	MaxConcurrentTokenInvalidatorWorkers *int32
	// MaxConcurrentTokenRequestorWorkers configures the number of worker threads for concurrent token requestor reconciliations
	MaxConcurrentTokenRequestorWorkers *int32
	// MaxConcurrentRootCAPublisherWorkers configures the number of worker threads for concurrent root ca publishing reconciliations
	MaxConcurrentRootCAPublisherWorkers *int32
	// Replicas is the number of replicas for the gardener-resource-manager deployment.
	Replicas *int32
	// RenewDeadline configures the renew deadline for leader election
	RenewDeadline *time.Duration
	// ResourceClass is used to filter resource resources
	ResourceClass *string
	// RetryPeriod configures the retry period for leader election
	RetryPeriod *time.Duration
	// SecretNameServerCA is the name of the server CA secret.
	SecretNameServerCA string
	// SyncPeriod configures the duration of how often existing resources should be synced
	SyncPeriod *time.Duration
	// TargetDiffersFromSourceCluster states whether the target cluster is a different one than the source cluster
	TargetDiffersFromSourceCluster bool
	// TargetDisableCache disables the cache for target cluster and always talk directly to the API server (defaults to false)
	TargetDisableCache *bool
	// WatchedNamespace restricts the gardener-resource-manager to only watch ManagedResources in the defined namespace.
	// If not set the gardener-resource-manager controller watches for ManagedResources in all namespaces
	WatchedNamespace *string
	// Version is the Kubernetes version for the Kubernetes components.
	Version *semver.Version
	// VPA contains information for configuring VerticalPodAutoscaler settings for the gardener-resource-manager deployment.
	VPA *VPAConfig
	// SchedulingProfile is the kube-scheduler profile configured for the Shoot.
	SchedulingProfile *gardencorev1beta1.SchedulingProfile
	// DefaultSeccompProfileEnabled specifies if the defaulting seccomp profile webhook of GRM should be enabled or not.
	DefaultSeccompProfileEnabled bool
}

// VPAConfig contains information for configuring VerticalPodAutoscaler settings for the gardener-resource-manager deployment.
type VPAConfig struct {
	// MinAllowed specifies the minimal amount of resources that will be recommended
	// for the container.
	MinAllowed corev1.ResourceList
}

func (r *resourceManager) Deploy(ctx context.Context) error {
	if r.values.TargetDiffersFromSourceCluster {
		r.secrets.shootAccess = r.newShootAccessSecret()
		if err := r.secrets.shootAccess.WithTokenExpirationDuration("24h").Reconcile(ctx, r.client); err != nil {
			return err
		}
	}

	fns := []func(context.Context) error{
		r.ensureServiceAccount,
		r.ensureRBAC,
		r.ensureService,
		r.ensureDeployment,
		r.ensurePodDisruptionBudget,
		r.ensureVPA,
	}

	if r.values.TargetDiffersFromSourceCluster {
		fns = append(fns, r.ensureShootResources)
		fns = append(fns, r.ensureNetworkPolicy)
	} else {
		fns = append(fns, r.ensureMutatingWebhookConfiguration)
	}

	for _, fn := range fns {
		if err := fn(ctx); err != nil {
			return err
		}
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObject(ctx, r.client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager-server", Namespace: r.namespace}})
}

func (r *resourceManager) Destroy(ctx context.Context) error {
	objectsToDelete := []client.Object{
		r.emptyPodDisruptionBudget(),
		r.emptyVPA(),
		r.emptyDeployment(),
		r.emptyService(),
		r.emptyServiceAccount(),
	}

	if r.values.TargetDiffersFromSourceCluster {
		if err := managedresources.DeleteForShoot(ctx, r.client, r.namespace, ManagedResourceName); err != nil {
			return err
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		if err := managedresources.WaitUntilDeleted(timeoutCtx, r.client, r.namespace, ManagedResourceName); err != nil {
			return err
		}

		objectsToDelete = append(objectsToDelete,
			r.newShootAccessSecret().Secret,
			r.emptyRoleInWatchedNamespace(),
			r.emptyRoleBindingInWatchedNamespace(),
		)
	} else {
		objectsToDelete = append(objectsToDelete,
			r.emptyMutatingWebhookConfiguration(),
			r.emptyClusterRole(),
			r.emptyClusterRoleBinding(),
		)
	}

	return kutil.DeleteObjects(
		ctx,
		r.client,
		objectsToDelete...,
	)
}

func (r *resourceManager) ensureRBAC(ctx context.Context) error {
	if r.values.TargetDiffersFromSourceCluster {
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
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, clusterRole, func() error {
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
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, clusterRoleBinding, func() error {
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
	role := r.emptyRoleInWatchedNamespace()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, role, func() error {
		role.Labels = r.getLabels()
		role.Rules = policies
		return nil
	})
	return err
}

func (r *resourceManager) emptyRoleInWatchedNamespace() *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: *r.values.WatchedNamespace}}
}

func (r *resourceManager) ensureRoleBinding(ctx context.Context) error {
	roleBinding := r.emptyRoleBindingInWatchedNamespace()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, roleBinding, func() error {
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

func (r *resourceManager) emptyRoleBindingInWatchedNamespace() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: *r.values.WatchedNamespace}}
}

func (r *resourceManager) ensureService(ctx context.Context) error {
	const (
		healthPortName = "health"
		serverPortName = "server"
	)

	service := r.emptyService()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, service, func() error {
		service.Labels = r.getLabels()
		service.Spec.Selector = appLabel()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		desiredPorts := []corev1.ServicePort{
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
			{
				Name:       serverPortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       serverServicePort,
				TargetPort: intstr.FromInt(serverPort),
			},
		}
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	})
	return err
}

func (r *resourceManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: r.namespace}}
}

func (r *resourceManager) ensureDeployment(ctx context.Context) error {
	deployment := r.emptyDeployment()

	secretServer, err := r.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  v1beta1constants.DeploymentNameGardenerResourceManager,
		DNSNames:                    kutil.DNSNamesForService(serviceName, r.namespace),
		CertType:                    secrets.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(r.values.SecretNameServerCA, secretsmanager.UseCurrentCA), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	priorityClassName := "gardener-system-critical-migration"
	if r.values.TargetDiffersFromSourceCluster {
		priorityClassName = v1beta1constants.PriorityClassNameShootControlPlane400
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, r.client, deployment, func() error {
		deployment.Labels = r.getLabels()

		deployment.Spec.Replicas = r.values.Replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: appLabel()}

		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(r.getDeploymentTemplateLabels(), r.getNetworkPolicyLabels(), map[string]string{
					resourcesv1alpha1.ProjectedTokenSkip: "true",
				}),
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									TopologyKey:   corev1.LabelHostname,
									LabelSelector: &metav1.LabelSelector{MatchLabels: r.getDeploymentTemplateLabels()},
								},
							},
						},
					},
				},
				PriorityClassName: priorityClassName,
				SecurityContext: &corev1.PodSecurityContext{
					// Workaround for https://github.com/kubernetes/kubernetes/issues/82573
					// Fixed with https://github.com/kubernetes/kubernetes/pull/89193 starting with Kubernetes 1.19
					// Adds the "nonroot" group as supplemental
					FSGroup: pointer.Int64(65532),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
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
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
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
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Scheme: "HTTP",
									Port:   intstr.FromInt(healthPort),
								},
							},
							InitialDelaySeconds: 10,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameAPIServerAccess,
								MountPath: volumeMountPathAPIServerAccess,
								ReadOnly:  true,
							},
							{
								MountPath: volumeMountPathCerts,
								Name:      volumeNameCerts,
								ReadOnly:  true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: volumeNameAPIServerAccess,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								DefaultMode: pointer.Int32(420),
								Sources: []corev1.VolumeProjection{
									{
										ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
											ExpirationSeconds: pointer.Int64(60 * 60 * 12),
											Path:              "token",
										},
									},
									{
										ConfigMap: &corev1.ConfigMapProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "kube-root-ca.crt",
											},
											Items: []corev1.KeyToPath{{
												Key:  "ca.crt",
												Path: "ca.crt",
											}},
										},
									},
									{
										DownwardAPI: &corev1.DownwardAPIProjection{
											Items: []corev1.DownwardAPIVolumeFile{{
												FieldRef: &corev1.ObjectFieldSelector{
													APIVersion: "v1",
													FieldPath:  "metadata.namespace",
												},
												Path: "namespace",
											}},
										},
									},
								},
							},
						},
					},
					{
						Name: volumeNameCerts,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  secretServer.Name,
								DefaultMode: pointer.Int32(420),
							},
						},
					},
				},
			},
		}

		if r.values.TargetDiffersFromSourceCluster {
			clusterCASecret, found := r.secretsManager.Get(v1beta1constants.SecretNameCACluster)
			if !found {
				return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
			}

			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeNameRootCA,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  clusterCASecret.Name,
						DefaultMode: pointer.Int32(420),
					},
				},
			})
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				MountPath: volumeMountPathRootCA,
				Name:      volumeNameRootCA,
				ReadOnly:  true,
			})

			if r.secrets.BootstrapKubeconfig != nil {
				deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: volumeNameBootstrapKubeconfig,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  r.secrets.BootstrapKubeconfig.Name,
							DefaultMode: pointer.Int32(420),
						},
					},
				})
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					MountPath: gutil.VolumeMountPathGenericKubeconfig,
					Name:      volumeNameBootstrapKubeconfig,
					ReadOnly:  true,
				})
			} else if r.secrets.shootAccess != nil {
				genericTokenKubeconfigSecret, found := r.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
				if !found {
					return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
				}

				utilruntime.Must(gutil.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, r.secrets.shootAccess.Secret.Name))
			}
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
	cmd = append(cmd, "--garbage-collector-sync-period=12h")
	cmd = append(cmd, fmt.Sprintf("--health-bind-address=:%v", healthPort))
	if r.values.MaxConcurrentHealthWorkers != nil {
		cmd = append(cmd, fmt.Sprintf("--health-max-concurrent-workers=%d", *r.values.MaxConcurrentHealthWorkers))
	}
	if r.values.MaxConcurrentTokenRequestorWorkers != nil {
		cmd = append(cmd, fmt.Sprintf("--token-requestor-max-concurrent-workers=%d", *r.values.MaxConcurrentTokenRequestorWorkers))
	}
	if r.values.MaxConcurrentTokenInvalidatorWorkers != nil {
		cmd = append(cmd, fmt.Sprintf("--token-invalidator-max-concurrent-workers=%d", *r.values.MaxConcurrentTokenInvalidatorWorkers))
	}
	if r.values.MaxConcurrentRootCAPublisherWorkers != nil {
		cmd = append(cmd, fmt.Sprintf("--root-ca-publisher-max-concurrent-workers=%d", *r.values.MaxConcurrentRootCAPublisherWorkers))
	}
	if r.values.MaxConcurrentRootCAPublisherWorkers != nil {
		if r.values.TargetDiffersFromSourceCluster {
			cmd = append(cmd, fmt.Sprintf("--root-ca-file=%s/%s", volumeMountPathRootCA, secrets.DataKeyCertificateBundle))
		} else {
			// default to using the CA cert from the mounted service account. Relevant when source=target cluster.
			// In this case, the CA cert of the source cluster is published.
			cmd = append(cmd, fmt.Sprintf("--root-ca-file=%s/ca.crt", volumeMountPathAPIServerAccess))
		}
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
	cmd = append(cmd, fmt.Sprintf("--port=%d", serverPort))
	cmd = append(cmd, fmt.Sprintf("--tls-cert-dir=%s", volumeMountPathCerts))
	if r.values.TargetDiffersFromSourceCluster {
		cmd = append(cmd, "--target-kubeconfig="+gutil.PathGenericKubeconfig)
	}
	if r.values.SchedulingProfile != nil && *r.values.SchedulingProfile != gardencorev1beta1.SchedulingProfileBalanced {
		cmd = append(cmd, "--pod-scheduler-name-webhook-enabled=true")
		cmd = append(cmd, fmt.Sprintf("--pod-scheduler-name-webhook-scheduler=%s", kubescheduler.BinPackingSchedulerName))
	}

	if r.values.DefaultSeccompProfileEnabled {
		cmd = append(cmd, "--seccomp-profile-webhook-enabled=true")
	}

	return cmd
}

func (r *resourceManager) ensureServiceAccount(ctx context.Context) error {
	serviceAccount := r.emptyServiceAccount()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, serviceAccount, func() error {
		serviceAccount.Labels = r.getLabels()
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		return nil
	})
	return err
}

func (r *resourceManager) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: r.namespace}}
}

func (r *resourceManager) ensureVPA(ctx context.Context) error {
	vpa := r.emptyVPA()
	vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
	controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, vpa, func() error {
		vpa.Labels = r.getLabels()
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameGardenerResourceManager,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
					MinAllowed:    r.values.VPA.MinAllowed,
					MaxAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("10G"),
					},
					ControlledValues: &controlledValues,
				},
			},
		}
		return nil
	})
	return err
}

func (r *resourceManager) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager-vpa", Namespace: r.namespace}}
}

func (r *resourceManager) ensurePodDisruptionBudget(ctx context.Context) error {
	pdb := r.emptyPodDisruptionBudget()
	maxUnavailable := intstr.FromInt(1)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, pdb, func() error {
		pdb.Labels = r.getLabels()
		pdb.Spec = policyv1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: r.getDeploymentTemplateLabels(),
			},
		}
		return nil
	})
	return err
}

func (r *resourceManager) emptyPodDisruptionBudget() *policyv1beta1.PodDisruptionBudget {
	return &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: r.namespace}}
}

func (r *resourceManager) ensureMutatingWebhookConfiguration(ctx context.Context) error {
	mutatingWebhookConfiguration := r.emptyMutatingWebhookConfiguration()

	secretServerCA, found := r.secretsManager.Get(r.values.SecretNameServerCA)
	if !found {
		return fmt.Errorf("secret %q not found", r.values.SecretNameServerCA)
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, mutatingWebhookConfiguration, func() error {
		mutatingWebhookConfiguration.Labels = utils.MergeStringMaps(appLabel(), map[string]string{
			v1beta1constants.LabelExcludeWebhookFromRemediation: "true",
		})
		mutatingWebhookConfiguration.Webhooks = getMutatingWebhookConfigurationWebhooks(
			r.buildWebhookNamespaceSelector(),
			secretServerCA,
			r.buildWebhookClientConfig,
			nil,
			r.values.DefaultSeccompProfileEnabled,
		)
		return nil
	})
	return err
}

func (r *resourceManager) emptyMutatingWebhookConfiguration() *admissionregistrationv1.MutatingWebhookConfiguration {
	suffix := ""
	if r.values.TargetDiffersFromSourceCluster {
		suffix = "-shoot"
	}
	return &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager" + suffix, Namespace: r.namespace}}
}

func (r *resourceManager) ensureShootResources(ctx context.Context) error {
	secretServerCA, found := r.secretsManager.Get(r.values.SecretNameServerCA)
	if !found {
		return fmt.Errorf("secret %q not found", r.values.SecretNameServerCA)
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		mutatingWebhookConfiguration = r.emptyMutatingWebhookConfiguration()

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "gardener.cloud:target:resource-manager",
				Annotations: map[string]string{resourcesv1alpha1.KeepObject: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      r.secrets.shootAccess.ServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}
	)

	mutatingWebhookConfiguration.Labels = appLabel()
	mutatingWebhookConfiguration.Webhooks = getMutatingWebhookConfigurationWebhooks(
		r.buildWebhookNamespaceSelector(),
		secretServerCA,
		r.buildWebhookClientConfig,
		r.values.SchedulingProfile,
		false,
	)

	data, err := registry.AddAllAndSerialize(
		mutatingWebhookConfiguration,
		clusterRoleBinding,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, r.client, r.namespace, ManagedResourceName, false, data)
}

func (r *resourceManager) ensureNetworkPolicy(ctx context.Context) error {
	networkPolicy := r.emptyNetworkPolicy()
	protocol := corev1.ProtocolTCP
	port := intstr.FromInt(serverPort)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.client, networkPolicy, func() error {
		networkPolicy.Labels = r.getLabels()
		networkPolicy.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: "Allows Egress from shoot's kube-apiserver pods to gardener-resource-manager pods.",
		}
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
					v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: appLabel(),
					},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: &protocol,
					Port:     &port,
				}},
			}},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		}
		return nil
	})
	return err
}

func (r *resourceManager) emptyNetworkPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-kube-apiserver-to-gardener-resource-manager", Namespace: r.namespace}}
}

func (r *resourceManager) newShootAccessSecret() *gutil.ShootAccessSecret {
	return gutil.NewShootAccessSecret(SecretNameShootAccess, r.namespace)
}

func getMutatingWebhookConfigurationWebhooks(
	namespaceSelector *metav1.LabelSelector,
	secretServerCA *corev1.Secret,
	buildClientConfigFn func(*corev1.Secret, string) admissionregistrationv1.WebhookClientConfig,
	schedulingProfile *gardencorev1beta1.SchedulingProfile,
	seccompWebhookEnabled bool,
) []admissionregistrationv1.MutatingWebhook {
	webhooks := []admissionregistrationv1.MutatingWebhook{
		GetTokenInvalidatorMutatingWebhook(namespaceSelector, secretServerCA, buildClientConfigFn),
		getProjectedTokenMountMutatingWebhook(namespaceSelector, secretServerCA, buildClientConfigFn),
	}

	if schedulingProfile != nil && *schedulingProfile == gardencorev1beta1.SchedulingProfileBinPacking {
		// pod scheduler name webhook should be active on all namespaces
		webhooks = append(webhooks, GetPodSchedulerNameMutatingWebhook(&metav1.LabelSelector{}, secretServerCA, buildClientConfigFn))
	}

	if seccompWebhookEnabled {
		webhooks = append(webhooks, GetSeccompProfileMutatingWebhook(namespaceSelector, secretServerCA, buildClientConfigFn))
	}

	return webhooks
}

// GetTokenInvalidatorMutatingWebhook returns the token-invalidator mutating webhook for the resourcemanager component for reuse
// between the component and integration tests.
func GetTokenInvalidatorMutatingWebhook(namespaceSelector *metav1.LabelSelector, secretServerCA *corev1.Secret, buildClientConfigFn func(*corev1.Secret, string) admissionregistrationv1.WebhookClientConfig) admissionregistrationv1.MutatingWebhook {
	var (
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
		sideEffect    = admissionregistrationv1.SideEffectClassNone
	)

	return admissionregistrationv1.MutatingWebhook{
		Name: "token-invalidator.resources.gardener.cloud",
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{corev1.GroupName},
				APIVersions: []string{corev1.SchemeGroupVersion.Version},
				Resources:   []string{"secrets"},
			},
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
		}},
		NamespaceSelector: namespaceSelector,
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenInvalidation},
		},
		ClientConfig:            buildClientConfigFn(secretServerCA, tokeninvalidator.WebhookPath),
		AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
		FailurePolicy:           &failurePolicy,
		MatchPolicy:             &matchPolicy,
		SideEffects:             &sideEffect,
		TimeoutSeconds:          pointer.Int32(10),
	}
}

func getProjectedTokenMountMutatingWebhook(namespaceSelector *metav1.LabelSelector, secretServerCA *corev1.Secret, buildClientConfigFn func(*corev1.Secret, string) admissionregistrationv1.WebhookClientConfig) admissionregistrationv1.MutatingWebhook {
	var (
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
		sideEffect    = admissionregistrationv1.SideEffectClassNone
	)

	return admissionregistrationv1.MutatingWebhook{
		Name: "projected-token-mount.resources.gardener.cloud",
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{corev1.GroupName},
				APIVersions: []string{corev1.SchemeGroupVersion.Version},
				Resources:   []string{"pods"},
			},
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
		}},
		NamespaceSelector: namespaceSelector,
		ObjectSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      resourcesv1alpha1.ProjectedTokenSkip,
					Operator: metav1.LabelSelectorOpDoesNotExist,
				},
				{
					Key:      v1beta1constants.LabelApp,
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"gardener-resource-manager"},
				},
			},
		},
		ClientConfig:            buildClientConfigFn(secretServerCA, projectedtokenmount.WebhookPath),
		AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
		FailurePolicy:           &failurePolicy,
		MatchPolicy:             &matchPolicy,
		SideEffects:             &sideEffect,
		TimeoutSeconds:          pointer.Int32(10),
	}
}

// GetPodSchedulerNameMutatingWebhook returns the pod-scheduler-name1 mutating webhook for the resourcemanager component for reuse
// between the component and integration tests.
func GetPodSchedulerNameMutatingWebhook(namespaceSelector *metav1.LabelSelector, secretServerCA *corev1.Secret, buildClientConfigFn func(*corev1.Secret, string) admissionregistrationv1.WebhookClientConfig) admissionregistrationv1.MutatingWebhook {
	var (
		failurePolicy = admissionregistrationv1.Ignore
		matchPolicy   = admissionregistrationv1.Exact
		sideEffect    = admissionregistrationv1.SideEffectClassNone
	)

	return admissionregistrationv1.MutatingWebhook{
		Name: "pod-scheduler-name.resources.gardener.cloud",
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{corev1.GroupName},
				APIVersions: []string{corev1.SchemeGroupVersion.Version},
				Resources:   []string{"pods"},
			},
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
		}},
		NamespaceSelector:       namespaceSelector,
		ObjectSelector:          &metav1.LabelSelector{},
		ClientConfig:            buildClientConfigFn(secretServerCA, podschedulername.WebhookPath),
		AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
		FailurePolicy:           &failurePolicy,
		MatchPolicy:             &matchPolicy,
		SideEffects:             &sideEffect,
		TimeoutSeconds:          pointer.Int32(10),
	}
}

// GetSeccompProfileMutatingWebhook returns the seccomp-profile mutating webhook for the resourcemanager component for reuse
// between the component and integration tests.
func GetSeccompProfileMutatingWebhook(
	namespaceSelector *metav1.LabelSelector,
	secretServerCA *corev1.Secret,
	buildClientConfigFn func(*corev1.Secret, string) admissionregistrationv1.WebhookClientConfig,
) admissionregistrationv1.MutatingWebhook {
	var (
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
		sideEffect    = admissionregistrationv1.SideEffectClassNone
	)

	return admissionregistrationv1.MutatingWebhook{
		Name: "seccomp-profile.resources.gardener.cloud",
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{corev1.GroupName},
				APIVersions: []string{corev1.SchemeGroupVersion.Version},
				Resources:   []string{"pods"},
			},
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
		}},
		NamespaceSelector: namespaceSelector,
		ObjectSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      resourcesv1alpha1.SeccompProfileSkip,
					Operator: metav1.LabelSelectorOpDoesNotExist,
				},
				{
					Key:      v1beta1constants.LabelApp,
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"gardener-resource-manager"},
				},
			},
		},
		ClientConfig:            buildClientConfigFn(secretServerCA, seccompprofile.WebhookPath),
		AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
		FailurePolicy:           &failurePolicy,
		MatchPolicy:             &matchPolicy,
		SideEffects:             &sideEffect,
		TimeoutSeconds:          pointer.Int32(10),
	}
}

func (r *resourceManager) buildWebhookNamespaceSelector() *metav1.LabelSelector {
	namespaceSelectorOperator := metav1.LabelSelectorOpIn
	if !r.values.TargetDiffersFromSourceCluster {
		namespaceSelectorOperator = metav1.LabelSelectorOpNotIn
	}

	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      v1beta1constants.GardenerPurpose,
			Operator: namespaceSelectorOperator,
			Values:   []string{metav1.NamespaceSystem, "kubernetes-dashboard"},
		}},
	}
}

func (r *resourceManager) buildWebhookClientConfig(secretServerCA *corev1.Secret, path string) admissionregistrationv1.WebhookClientConfig {
	clientConfig := admissionregistrationv1.WebhookClientConfig{CABundle: secretServerCA.Data[secrets.DataKeyCertificateBundle]}

	if r.values.TargetDiffersFromSourceCluster {
		clientConfig.URL = pointer.String(fmt.Sprintf("https://%s.%s:%d%s", serviceName, r.namespace, serverServicePort, path))
	} else {
		clientConfig.Service = &admissionregistrationv1.ServiceReference{
			Name:      serviceName,
			Namespace: r.namespace,
			Path:      &path,
		}
	}

	return clientConfig
}

func (r *resourceManager) getLabels() map[string]string {
	if r.values.TargetDiffersFromSourceCluster {
		return utils.MergeStringMaps(appLabel(), map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		})
	}

	return appLabel()
}

func (r *resourceManager) getDeploymentTemplateLabels() map[string]string {
	role := v1beta1constants.GardenRoleSeed
	if r.values.TargetDiffersFromSourceCluster {
		role = v1beta1constants.GardenRoleControlPlane
	}

	return utils.MergeStringMaps(appLabel(), map[string]string{
		v1beta1constants.GardenRole: role,
	})
}

func (r *resourceManager) getNetworkPolicyLabels() map[string]string {
	if r.values.TargetDiffersFromSourceCluster {
		return map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToShootAPIServer:   v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyFromShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToSeedAPIServer:    v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyFromPrometheus:     v1beta1constants.LabelNetworkPolicyAllowed,
		}
	}

	return map[string]string{
		v1beta1constants.LabelNetworkPolicyFromPrometheus: v1beta1constants.LabelNetworkPolicyAllowed,
	}
}

func appLabel() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: LabelValue,
	}
}

var (
	// IntervalWaitForDeployment is the interval used while waiting for the Deployments to become healthy
	// or deleted.
	IntervalWaitForDeployment = 5 * time.Second
	// TimeoutWaitForDeployment is the timeout used while waiting for the Deployments to become healthy
	// or deleted.
	TimeoutWaitForDeployment = 5 * time.Minute
)

// Wait signals whether a deployment is ready or needs more time to be deployed.
func (r *resourceManager) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForDeployment)
	defer cancel()

	return retry.Until(timeoutCtx, IntervalWaitForDeployment, func(ctx context.Context) (done bool, err error) {
		deployment := r.emptyDeployment()
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckDeployment(deployment); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}

// WaitCleanup for destruction to finish and component to be fully removed. Gardener-Resource-manager does not need to wait for cleanup.
func (r *resourceManager) WaitCleanup(_ context.Context) error { return nil }

// GetReplicas returns Replicas field in the Values.
func (r *resourceManager) GetReplicas() *int32 { return r.values.Replicas }

// SetReplicas sets the Replicas field in the Values.
func (r *resourceManager) SetReplicas(replicas *int32) { r.values.Replicas = replicas }

// SetSecrets sets the secrets for the gardener-resource-manager.
func (r *resourceManager) SetSecrets(s Secrets) { r.secrets = s }

// Secrets is collection of secrets for the gardener-resource-manager.
type Secrets struct {
	// BootstrapKubeconfig is the kubeconfig of the gardener-resource-manager used during the bootstrapping process. Its
	// token requestor controller will request a JWT token for itself with this kubeconfig.
	BootstrapKubeconfig *component.Secret

	shootAccess *gutil.ShootAccessSecret
}
