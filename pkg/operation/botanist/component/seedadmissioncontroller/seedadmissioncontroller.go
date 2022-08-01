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

package seedadmissioncontroller

import (
	"context"
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensioncrds"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Name is used as metadata.name of the ServiceAccount, ManagedResource,
	// ClusterRole, ClusterRoleBinding, Service, Deployment and ValidatingWebhookConfiguration
	// of the seed admission controller.
	Name = "gardener-seed-admission-controller"

	managedResourceName = Name
	deploymentName      = Name
	containerName       = Name

	metricsPort     = 8080
	healthPort      = 8081
	port            = 10250
	volumeName      = Name + "-tls"
	volumeMountPath = "/srv/gardener-seed-admission-controller"

	defaultReplicas = int32(3)
)

// New creates a new instance of DeployWaiter for the gardener-seed-admission-controller.
func New(c client.Client, namespace string, secretsManager secretsmanager.Interface, image string) component.DeployWaiter {
	return &gardenerSeedAdmissionController{
		client:         c,
		namespace:      namespace,
		secretsManager: secretsManager,
		image:          image,
	}
}

type gardenerSeedAdmissionController struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	image          string
}

func (g *gardenerSeedAdmissionController) Deploy(ctx context.Context) error {
	replicas, err := g.getReplicas(ctx)
	if err != nil {
		return err
	}

	caSecret, found := g.secretsManager.Get(v1beta1constants.SecretNameCASeed)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCASeed)
	}

	serverSecret, err := g.secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:                        Name + "-server",
		CommonName:                  Name,
		DNSNames:                    kutil.DNSNamesForService(Name, g.namespace),
		CertType:                    secretutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed, secretsmanager.UseCurrentCA), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   Name,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{apiextensionsv1.SchemeGroupVersion.Group},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list"},
				},
				{
					APIGroups: []string{druidv1alpha1.GroupVersion.Group},
					Resources: []string{"etcds"},
					Verbs:     []string{"get", "list"},
				},
				{
					APIGroups: []string{extensionsv1alpha1.SchemeGroupVersion.Group},
					Resources: []string{
						"backupbuckets",
						"backupentries",
						"bastions",
						"containerruntimes",
						"controlplanes",
						"dnsrecords",
						"extensions",
						"infrastructures",
						"networks",
						"operatingsystemconfigs",
						"workers",
						"clusters",
					},
					Verbs: []string{"get", "list"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   Name,
				Labels: getLabels(),
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

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: getLabels(),
				Ports: []corev1.ServicePort{
					{
						Name:       "metrics",
						Protocol:   corev1.ProtocolTCP,
						Port:       metricsPort,
						TargetPort: intstr.FromInt(metricsPort),
					},
					{
						Name:       "health",
						Port:       healthPort,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(healthPort),
					},
					{
						Name:       "web",
						Port:       443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(port),
					},
				},
			},
		}

		// if maxUnavailable would not be set, new pods don't come up in small seed clusters
		// (due to the pod anti affinity new pods are stuck in pending state)
		maxUnavailable = intstr.FromInt(1)
		deployment     = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(1),
				Replicas:             &replicas,
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxUnavailable: &maxUnavailable,
					},
				},
				Selector: &metav1.LabelSelector{MatchLabels: getLabels()},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getLabels(),
					},
					Spec: corev1.PodSpec{
						PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem900,
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: 100,
										PodAffinityTerm: corev1.PodAffinityTerm{
											TopologyKey:   corev1.LabelHostname,
											LabelSelector: &metav1.LabelSelector{MatchLabels: getLabels()},
										},
									},
								},
							},
						},
						ServiceAccountName: serviceAccount.Name,
						Containers: []corev1.Container{{
							Name:            containerName,
							Image:           g.image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/gardener-seed-admission-controller",
								fmt.Sprintf("--port=%d", port),
								fmt.Sprintf("--tls-cert-dir=%s", volumeMountPath),
								fmt.Sprintf("--metrics-bind-address=:%d", metricsPort),
								fmt.Sprintf("--health-bind-address=:%d", healthPort),
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "metrics",
									ContainerPort: metricsPort,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: int32(port),
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
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
								InitialDelaySeconds: 5,
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
							VolumeMounts: []corev1.VolumeMount{{
								Name:      volumeName,
								MountPath: volumeMountPath,
								ReadOnly:  true,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: serverSecret.Name,
								},
							},
						}},
					},
				},
			},
		}

		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
			},
		}

		updateMode = vpaautoscalingv1.UpdateModeAuto
		vpa        = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name + "-vpa",
				Namespace: g.namespace,
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

		validatingWebhookConfiguration = GetValidatingWebhookConfig(caSecret.Data[secretutils.DataKeyCertificateBundle], service)
	)

	resources, err := registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		service,
		deployment,
		podDisruptionBudget,
		vpa,
		validatingWebhookConfiguration,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, g.client, g.namespace, managedResourceName, false, resources)
}

func (g *gardenerSeedAdmissionController) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, g.client, g.namespace, managedResourceName)
}

// GetValidatingWebhookConfig returns the ValidatingWebhookConfiguration for the seedadmissioncontroller component for
// reuse between the component and integration tests.
func GetValidatingWebhookConfig(caBundle []byte, webhookClientService *corev1.Service) *admissionregistrationv1.ValidatingWebhookConfiguration {
	var (
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
		sideEffect    = admissionregistrationv1.SideEffectClassNone
	)
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   Name,
			Labels: getLabels(),
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name: "crds.seed.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{apiextensionsv1.GroupName},
					APIVersions: []string{apiextensionsv1beta1.SchemeGroupVersion.Version, apiextensionsv1.SchemeGroupVersion.Version},
					Resources:   []string{"customresourcedefinitions"},
				},
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
			}},
			FailurePolicy:     &failurePolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: extensioncrds.ObjectSelector,
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caBundle,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookClientService.Name,
					Namespace: webhookClientService.Namespace,
					Path:      pointer.String(extensioncrds.WebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			MatchPolicy:             &matchPolicy,
			SideEffects:             &sideEffect,
			TimeoutSeconds:          pointer.Int32(10),
		}, {
			Name: "crs.seed.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{druidv1alpha1.GroupVersion.Group},
						APIVersions: []string{druidv1alpha1.GroupVersion.Version},
						Resources:   []string{"etcds"},
					},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
				},
				{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{extensionsv1alpha1.SchemeGroupVersion.Group},
						APIVersions: []string{extensionsv1alpha1.SchemeGroupVersion.Version},
						Resources: []string{
							"backupbuckets",
							"backupentries",
							"bastions",
							"containerruntimes",
							"controlplanes",
							"dnsrecords",
							"extensions",
							"infrastructures",
							"networks",
							"operatingsystemconfigs",
							"workers",
						},
					},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
				},
			},
			FailurePolicy:     &failurePolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caBundle,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookClientService.Name,
					Namespace: webhookClientService.Namespace,
					Path:      pointer.String(extensioncrds.WebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			MatchPolicy:             &matchPolicy,
			SideEffects:             &sideEffect,
			TimeoutSeconds:          pointer.Int32(10),
		}, {
			Name: "validation.extensions.etcd.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{druidv1alpha1.GroupVersion.Group},
						APIVersions: []string{druidv1alpha1.GroupVersion.Version},
						Resources:   []string{"etcds"},
					},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				},
			},
			FailurePolicy:     &failurePolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caBundle,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookClientService.Name,
					Namespace: webhookClientService.Namespace,
					Path:      pointer.String(extensionresources.EtcdWebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			MatchPolicy:             &matchPolicy,
			SideEffects:             &sideEffect,
			TimeoutSeconds:          pointer.Int32(10),
		}},
	}

	webhookConfig.Webhooks = append(webhookConfig.Webhooks, getWebhooks(caBundle, webhookClientService)...)
	return webhookConfig
}

func getWebhooks(caBundle []byte, webhookClientService *corev1.Service) []admissionregistrationv1.ValidatingWebhook {
	var (
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
		sideEffect    = admissionregistrationv1.SideEffectClassNone
		webhooks      []admissionregistrationv1.ValidatingWebhook
	)

	resources := map[string]string{
		"backupbuckets":          extensionresources.BackupBucketWebhookPath,
		"backupentries":          extensionresources.BackupEntryWebhookPath,
		"bastions":               extensionresources.BastionWebhookPath,
		"containerruntimes":      extensionresources.ContainerRuntimeWebhookPath,
		"controlplanes":          extensionresources.ControlPlaneWebhookPath,
		"dnsrecords":             extensionresources.DNSRecordWebhookPath,
		"extensions":             extensionresources.ExtensionWebhookPath,
		"infrastructures":        extensionresources.InfrastructureWebhookPath,
		"networks":               extensionresources.NetworkWebhookPath,
		"operatingsystemconfigs": extensionresources.OperatingSystemConfigWebhookPath,
		"workers":                extensionresources.WorkerWebhookPath,
	}

	resourcesName := []string{"backupbuckets", "backupentries", "bastions", "containerruntimes", "controlplanes", "dnsrecords", "extensions", "infrastructures", "networks", "operatingsystemconfigs", "workers"}

	for _, resource := range resourcesName {
		webhook := admissionregistrationv1.ValidatingWebhook{
			Name: "validation.extensions." + resource + ".admission.core.gardener.cloud",
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{extensionsv1alpha1.SchemeGroupVersion.Group},
						APIVersions: []string{extensionsv1alpha1.SchemeGroupVersion.Version},
						Resources: []string{
							resource,
						},
					},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				},
			},
			FailurePolicy:     &failurePolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caBundle,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookClientService.Name,
					Namespace: webhookClientService.Namespace,
					Path:      pointer.String(resources[resource]),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			MatchPolicy:             &matchPolicy,
			SideEffects:             &sideEffect,
			TimeoutSeconds:          pointer.Int32(10),
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  "gardener",
		v1beta1constants.LabelRole: "seed-admission-controller",
	}
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (g *gardenerSeedAdmissionController) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, managedResourceName)
}

func (g *gardenerSeedAdmissionController) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, managedResourceName)
}

func (g *gardenerSeedAdmissionController) getReplicas(ctx context.Context) (int32, error) {
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))

	err := g.client.List(ctx, nodeList, client.Limit(defaultReplicas))
	if err != nil {
		return 0, err
	}

	nodeCount := int32(len(nodeList.Items))
	if nodeCount < defaultReplicas {
		return nodeCount, nil
	}

	return defaultReplicas, nil
}
