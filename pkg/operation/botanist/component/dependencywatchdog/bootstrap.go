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

package dependencywatchdog

import (
	"context"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	restarterapi "github.com/gardener/dependency-watchdog/pkg/restarter/api"
	scalerapi "github.com/gardener/dependency-watchdog/pkg/scaler/api"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Role is a string alias type.
type Role string

const (
	// RoleEndpoint is a constant for the 'endpoint' role of the dependency-watchdog.
	RoleEndpoint Role = "endpoint"
	// RoleProbe is a constant for the 'probe' role of the dependency-watchdog.
	RoleProbe Role = "probe"

	name            = "dependency-watchdog"
	volumeName      = "config"
	volumeMountPath = "/etc/dependency-watchdog/config"
	configFileName  = "dep-config.yaml"

	// ManagedResourceDependencyWatchdogEndpoint is the name of the dependency-watchdog-endpoint managed resource.
	ManagedResourceDependencyWatchdogEndpoint = "dependency-watchdog-endpoint"
	// ManagedResourceDependencyWatchdogProbe is the name of the dependency-watchdog-probe managed resource.
	ManagedResourceDependencyWatchdogProbe = "dependency-watchdog-probe"
)

// BootstrapperValues contains dependency-watchdog values.
type BootstrapperValues struct {
	Role Role
	ValuesEndpoint
	ValuesProbe
	Image string
}

// ValuesEndpoint contains the service dependants of dependency-watchdog.
type ValuesEndpoint struct {
	ServiceDependants restarterapi.ServiceDependants
}

// ValuesProbe contains the probe dependants list of dependency-watchdog.
type ValuesProbe struct {
	ProbeDependantsList scalerapi.ProbeDependantsList
}

// NewBootstrapper creates a new instance of DeployWaiter for the dependency-watchdog.
func NewBootstrapper(
	client client.Client,
	namespace string,
	values BootstrapperValues,
) component.DeployWaiter {
	return &bootstrapper{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type bootstrapper struct {
	client    client.Client
	namespace string
	values    BootstrapperValues
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	var (
		config              string
		vpaMinAllowedMemory string
		err                 error
	)

	switch b.values.Role {
	case RoleEndpoint:
		config, err = restarterapi.Encode(&b.values.ValuesEndpoint.ServiceDependants)
		if err != nil {
			return err
		}
		vpaMinAllowedMemory = "25Mi"

	case RoleProbe:
		config, err = scalerapi.Encode(&b.values.ValuesProbe.ProbeDependantsList)
		if err != nil {
			return err
		}
		vpaMinAllowedMemory = "50Mi"
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name() + "-config",
			Namespace: b.namespace,
			Labels:    map[string]string{v1beta1constants.LabelApp: b.name()},
		},
		Data: map[string]string{configFileName: config},
	}
	utilruntime.Must(kutil.MakeUnique(configMap))

	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.name(),
				Namespace: b.namespace,
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("gardener.cloud:%s:cluster-role", b.name()),
			},
			Rules: b.clusterRoleRules(),
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("gardener.cloud:%s:cluster-role-binding", b.name()),
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

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("gardener.cloud:%s:role", b.name()),
				Namespace: b.namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints", "events"},
					Verbs:     []string{"create", "get", "update", "patch"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("gardener.cloud:%s:role-binding", b.name()),
				Namespace: b.namespace,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.name(),
				Namespace: b.namespace,
				Labels:    b.getLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector:             &metav1.LabelSelector{MatchLabels: b.getLabels()},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: b.podLabels(),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:             v1beta1constants.PriorityClassNameSeedSystem800,
						ServiceAccountName:            serviceAccount.Name,
						TerminationGracePeriodSeconds: pointer.Int64(5),
						Containers: []corev1.Container{{
							Name:            name,
							Image:           b.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         b.containerCommand(),
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9643,
								Protocol:      corev1.ProtocolTCP,
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      volumeName,
								MountPath: volumeMountPath,
								ReadOnly:  true,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						}},
					},
				},
			},
		}

		updateMode = vpaautoscalingv1.UpdateModeAuto
		vpa        = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.name() + "-vpa",
				Namespace: b.namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &updateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("25m"),
							corev1.ResourceMemory: resource.MustParse(vpaMinAllowedMemory),
						},
					}},
				},
			},
		}
	)

	utilruntime.Must(references.InjectAnnotations(deployment))

	resources, err := registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		role,
		roleBinding,
		configMap,
		deployment,
		vpa,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, b.client, b.namespace, b.name(), false, resources)
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, b.client, b.namespace, b.name())
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *bootstrapper) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, b.name())
}

func (b *bootstrapper) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, b.name())
}

func (b *bootstrapper) name() string {
	return fmt.Sprintf("%s-%s", name, b.values.Role)
}

func (b *bootstrapper) getLabels() map[string]string {
	return map[string]string{v1beta1constants.LabelApp: b.name()}
}

func (b *bootstrapper) clusterRoleRules() []rbacv1.PolicyRule {
	switch b.values.Role {
	case RoleEndpoint:
		return []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch", "delete"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{"coordination.k8s.io"},
				ResourceNames: []string{"dependency-watchdog"},
				Resources:     []string{"leases"},
				Verbs:         []string{"get", "watch", "update"},
			},
		}

	case RoleProbe:
		return []rbacv1.PolicyRule{
			{
				APIGroups: []string{"extensions.gardener.cloud"},
				Resources: []string{"clusters"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "deployments/scale"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{"coordination.k8s.io"},
				ResourceNames: []string{"dependency-watchdog-probe"},
				Resources:     []string{"leases"},
				Verbs:         []string{"get", "watch", "update"},
			},
		}
	}

	return nil
}

func (b *bootstrapper) podLabels() map[string]string {
	switch b.values.Role {
	case RoleEndpoint:
		return utils.MergeStringMaps(b.getLabels(), map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:           v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToSeedAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})

	case RoleProbe:
		return utils.MergeStringMaps(b.getLabels(), map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:                v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToSeedAPIServer:      v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToAllShootAPIServers: v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:     v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks:    v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	return nil
}

func (b *bootstrapper) containerCommand() []string {
	switch b.values.Role {
	case RoleEndpoint:
		return []string{
			"/usr/local/bin/dependency-watchdog",
			fmt.Sprintf("--config-file=%s/%s", volumeMountPath, configFileName),
			"--deployed-namespace=" + b.namespace,
			"--watch-duration=5m",
		}

	case RoleProbe:
		return []string{
			"/usr/local/bin/dependency-watchdog",
			"probe",
			fmt.Sprintf("--config-file=%s/%s", volumeMountPath, configFileName),
			"--deployed-namespace=" + b.namespace,
			"--qps=20.0",
			"--burst=100",
			"--v=4",
		}
	}

	return nil
}
