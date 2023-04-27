// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fluentoperator

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// OperatorManagedResourceName is the name of the Fluent Operator managed resource.
	OperatorManagedResourceName = "fluent-operator"
	name                        = "fluent-operator"
	roleName                    = "gardener.cloud:logging:fluent-operator"
	vpaName                     = "fluent-operator-vpa"
)

var (
	protocolTCP = utils.ProtocolPtr(corev1.ProtocolTCP)
	lokiPort    = utils.IntStrPtrFromInt(3100)
)

// Values keeps values for the Fluent Operator.
type Values struct {
	// OperatorImage is the image of the Fluent Operator
	OperatorImage string
	// FluentBitImage is the Fluent-bit image
	OperatorPriorityClassName string
}

type fluentOperator struct {
	client    client.Client
	namespace string
	values    Values
}

// NewFluentOperator creates a new instance of Fluent Operator.
func NewFluentOperator(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &fluentOperator{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (f *fluentOperator) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: f.namespace,
				Labels:    f.getLabels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   roleName,
				Labels: f.getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets", "statefulsets"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets", "configmaps", "serviceaccounts", "services", "namespaces"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"fluentbit.fluent.io"},
					Resources: []string{"fluentbits", "clusterfluentbitconfigs", "clusterfilters", "clusterinputs", "clusteroutputs", "clusterparsers", "collectors", "fluentbitconfigs", "filters", "outputs", "parsers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterrolebindings", "clusterroles"},
					Verbs:     []string{"get", "list", "watch", "create"},
				},
				{
					APIGroups: []string{"extensions.gardener.cloud"},
					Resources: []string{"clusters"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: f.getLabels(),
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
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameFluentOperator,
				Namespace: f.namespace,
				Labels:    f.getLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(2),
				Replicas:             pointer.Int32(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: f.getLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: f.getLabels(),
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						PriorityClassName:  f.values.OperatorPriorityClassName,
						Containers: []corev1.Container{
							{
								Name:            name,
								Image:           f.values.OperatorImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args:            []string{"--disable-component-controllers", "fluentd"},
								Env:             f.computeEnv(),
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "env",
										MountPath: "/fluent-operator",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "env",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
			},
		}
		vpaUpdateMode = vpaautoscalingv1.UpdateModeAuto
		vpa           = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: f.namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
			},
		}
	)

	resources := []client.Object{
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		deployment,
		vpa,
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, f.client, f.namespace, OperatorManagedResourceName, false, serializedResources)
}

func (f *fluentOperator) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, f.client, f.namespace, OperatorManagedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (f *fluentOperator) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, f.client, f.namespace, OperatorManagedResourceName)
}

func (f *fluentOperator) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, f.client, f.namespace, OperatorManagedResourceName)
}

func (f *fluentOperator) computeEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.namespace",
				},
			},
		},
	}
}

func (f *fluentOperator) getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:                             name,
		v1beta1constants.LabelRole:                            v1beta1constants.LabelLogging,
		v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleLogging,
		v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
	}
}

func getFluentBitLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:                                         v1beta1constants.DaemonSetNameFluentBit,
		v1beta1constants.LabelRole:                                        v1beta1constants.LabelLogging,
		v1beta1constants.GardenRole:                                       v1beta1constants.GardenRoleLogging,
		v1beta1constants.LabelNetworkPolicyToDNS:                          v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:             v1beta1constants.LabelNetworkPolicyAllowed,
		"networking.resources.gardener.cloud/to-logging-tcp-3100":         v1beta1constants.LabelNetworkPolicyAllowed,
		"networking.resources.gardener.cloud/to-all-shoots-loki-tcp-3100": v1beta1constants.LabelNetworkPolicyAllowed,
	}
}
