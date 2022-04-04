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

package gardenerkubescheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/configurator"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
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
	containerName                         = "kube-scheduler"
	portNameMetrics                       = "metrics"
	dataKeyComponentConfig                = "config.yaml"
	kubeSchedulerClusterRoleBindingName   = "gardener.cloud:kube-scheduler"
	volumeSchedulerClusterRoleBindingName = "gardener.cloud:volume-scheduler"
	roleBindingName                       = "gardener.cloud:kube-scheduler:extension-apiserver-authentication-reader"
	roleName                              = "extension-apiserver-authentication-reader"
	kubeSchedulerClusterRoleName          = "system:kube-scheduler"
	volumeSchedulerClusterRoleName        = "system:volume-scheduler"
	webhookName                           = "kube-scheduler.scheduling.gardener.cloud"
	volumeMountPathConfig                 = "/var/lib/kube-scheduler-config"
)

// New creates a new instance of DeployWaiter for the kube-scheduler.
// It requires Seed cluster with version 1.18 or 1.19.
func New(
	client client.Client,
	namespace string,
	image *imagevector.Image,
	version *semver.Version,
	config configurator.Configurator,
	webhookClientConfig *admissionregistrationv1.WebhookClientConfig,
) (
	component.DeployWaiter,
	error,
) {
	if client == nil {
		return nil, errors.New("client is required")
	}

	if len(namespace) == 0 {
		return nil, errors.New("namespace is required")
	}

	if namespace == v1beta1constants.GardenNamespace {
		return nil, errors.New("namespace cannot be 'garden'")
	}

	s := &kubeScheduler{
		client:              client,
		namespace:           namespace,
		image:               image,
		version:             version,
		config:              config,
		webhookClientConfig: webhookClientConfig,
	}

	return s, nil
}

type kubeScheduler struct {
	client              client.Client
	namespace           string
	image               *imagevector.Image
	version             *semver.Version
	config              configurator.Configurator
	webhookClientConfig *admissionregistrationv1.WebhookClientConfig
}

func (k *kubeScheduler) Deploy(ctx context.Context) error {
	if k.config == nil {
		return errors.New("config is required")
	}

	if k.image == nil || len(k.image.String()) == 0 {
		return errors.New("image is required")
	}

	if k.webhookClientConfig == nil {
		return errors.New("webhookClientConfig is required")
	}

	componentConfigYAML, err := k.config.Config()
	if err != nil {
		return fmt.Errorf("generate component config failed: %w", err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: k.namespace,
			Labels:    getLabels(),
		},
		Data: map[string]string{dataKeyComponentConfig: componentConfigYAML},
	}
	utilruntime.Must(kutil.MakeUnique(configMap))

	const (
		port             int32  = 10259
		configVolumeName string = "config"
	)

	var (
		updateMode   = vpaautoscalingv1.UpdateModeAuto
		minAvailable = intstr.FromInt(1)

		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   k.namespace,
			Labels: getLabels(),
		}}
		kubeSchedulerClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   kubeSchedulerClusterRoleBindingName,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     kubeSchedulerClusterRoleName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      Name,
				Namespace: k.namespace,
			}},
		}
		volumeSchedulerClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   volumeSchedulerClusterRoleBindingName,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     volumeSchedulerClusterRoleName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      Name,
				Namespace: k.namespace,
			}},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: k.namespace,
				Labels:    getLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(2),
				RevisionHistoryLimit: pointer.Int32(1),
				Selector:             &metav1.LabelSelector{MatchLabels: getLabels()},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(),
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey:   corev1.LabelHostname,
										LabelSelector: &metav1.LabelSelector{MatchLabels: getLabels()},
									},
								}},
							},
						},
						ServiceAccountName: Name,
						Containers: []corev1.Container{
							{
								Name:            containerName,
								Image:           k.image.String(),
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         k.command(port),
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/healthz",
											Scheme: corev1.URISchemeHTTPS,
											Port:   intstr.FromInt(int(port)),
										},
									},
									SuccessThreshold:    1,
									FailureThreshold:    2,
									InitialDelaySeconds: 15,
									PeriodSeconds:       10,
									TimeoutSeconds:      15,
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          portNameMetrics,
										ContainerPort: port,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("23m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      configVolumeName,
										MountPath: volumeMountPathConfig,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: configVolumeName,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: k.namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     roleName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      Name,
				Namespace: k.namespace,
			}},
		}
		leaseRole = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: k.namespace,
				Labels:    getLabels(),
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:     []string{"create"},
				Resources: []string{"leases"},
				APIGroups: []string{coordinationv1.SchemeGroupVersion.Group},
			}, {
				Verbs:         []string{"get", "update"},
				Resources:     []string{"leases"},
				APIGroups:     []string{coordinationv1.SchemeGroupVersion.Group},
				ResourceNames: []string{Name},
			}},
		}
		leaseRoleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: k.namespace,
				Labels:    getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      Name,
				Namespace: k.namespace,
			}},
		}
		webhook = GetMutatingWebhookConfig(*k.webhookClientConfig)
		vpa     = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: k.namespace,
				Labels:    getLabels(),
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
			},
		}
		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: k.namespace,
				Labels:    getLabels(),
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MinAvailable: &minAvailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
			},
		}

		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	)

	utilruntime.Must(references.InjectAnnotations(deployment))

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, namespace, func() error {
		namespace.Labels = utils.MergeStringMaps(namespace.Labels, getLabels())
		return nil
	}); err != nil {
		return fmt.Errorf("update of Namespace failed: %w", err)
	}

	resources, err := registry.AddAllAndSerialize(
		kubeSchedulerClusterRoleBinding,
		volumeSchedulerClusterRoleBinding,
		roleBinding,
		serviceAccount,
		leaseRole,
		leaseRoleBinding,
		configMap,
		deployment,
		webhook,
		vpa,
		podDisruptionBudget,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, k.client, k.namespace, "gardener-kube-scheduler", false, resources)
}

// GetMutatingWebhookConfig returns the MutatingWebhookConfiguration for the seedadmissioncontroller component for
// reuse between the component and integration tests.
func GetMutatingWebhookConfig(clientConfig admissionregistrationv1.WebhookClientConfig) *admissionregistrationv1.MutatingWebhookConfiguration {
	var (
		failPolicy         = admissionregistrationv1.Ignore
		matchPolicy        = admissionregistrationv1.Exact
		reinvocationPolicy = admissionregistrationv1.NeverReinvocationPolicy
		timeout            = int32(2)
		sideEffects        = admissionregistrationv1.SideEffectClassNone
		scope              = admissionregistrationv1.NamespacedScope
	)

	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   webhookName,
			Labels: getLabels(),
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name:         webhookName,
			ClientConfig: clientConfig,
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{corev1.GroupName},
					APIVersions: []string{corev1.SchemeGroupVersion.Version},
					Scope:       &scope,
					Resources:   []string{"pods"},
				},
			}},
			FailurePolicy: &failPolicy,
			MatchPolicy:   &matchPolicy,
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
				},
			},
			ObjectSelector:          &metav1.LabelSelector{},
			SideEffects:             &sideEffects,
			TimeoutSeconds:          &timeout,
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			ReinvocationPolicy:      &reinvocationPolicy,
		}},
	}
}

func getLabels() map[string]string {
	return map[string]string{
		"app":  "kubernetes",
		"role": "scheduler",
	}
}

func (k *kubeScheduler) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(k.client.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: k.namespace}}))
}

func (k *kubeScheduler) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, k.client, k.namespace, "gardener-kube-scheduler")
}

func (k *kubeScheduler) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(
		ctx,
		k.client,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: k.namespace}},
		time.Second*2,
	)
}

func (k *kubeScheduler) command(port int32) []string {
	command := []string{
		"/usr/local/bin/kube-scheduler",
		fmt.Sprintf("--config=%s/%s", volumeMountPathConfig, dataKeyComponentConfig),
		fmt.Sprintf("--secure-port=%d", port),
		"--v=2",
	}

	if version.ConstraintK8sLessEqual122.Check(k.version) {
		command = append(command, "--port=0")
	}

	return command
}
