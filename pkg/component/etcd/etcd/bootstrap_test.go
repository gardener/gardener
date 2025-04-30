// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"context"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd/etcd"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Etcd", func() {
	var (
		c            client.Client
		sm           secretsmanager.Interface
		bootstrapper component.DeployWaiter
		etcdConfig   *gardenletconfigv1alpha1.ETCDConfig
		consistOf    func(...client.Object) types.GomegaMatcher

		ctx                      = context.Background()
		namespace                = "shoot--foo--bar"
		etcdDruidImage           = "etcd/druid:1.2.3"
		imageVectorOverwrite     *string
		imageVectorOverwriteFull = ptr.To("some overwrite")
		secretNameCA             = "ca"

		priorityClassName = "some-priority-class"

		featureGates map[string]bool

		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource

		managedResourceName       = "etcd-druid"
		managedResourceSecretName = "managedresource-" + managedResourceName
	)

	JustBeforeEach(func() {
		etcdConfig = &gardenletconfigv1alpha1.ETCDConfig{
			ETCDController: &gardenletconfigv1alpha1.ETCDController{
				Workers: ptr.To[int64](25),
			},
			CustodianController: &gardenletconfigv1alpha1.CustodianController{
				Workers: ptr.To[int64](3),
			},
			BackupCompactionController: &gardenletconfigv1alpha1.BackupCompactionController{
				Workers:                   ptr.To[int64](3),
				EnableBackupCompaction:    ptr.To(true),
				EventsThreshold:           ptr.To[int64](1000000),
				MetricsScrapeWaitDuration: &metav1.Duration{Duration: time.Second * 60},
				ActiveDeadlineDuration:    &metav1.Duration{Duration: time.Hour * 3},
			},
			FeatureGates: featureGates,
		}

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
		sm = fakesecretsmanager.New(c, namespace)

		// Create CA secret for etcd-components webhook handler
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretNameCA, Namespace: namespace}})).To(Succeed())

		bootstrapper = NewBootstrapper(c, namespace, etcdConfig, etcdDruidImage, imageVectorOverwrite, sm, secretNameCA, priorityClassName)

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		var (
			expectedResources []client.Object

			configMapName = "etcd-druid-imagevector-overwrite-4475dd36"

			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-druid",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
					},
				},
				AutomountServiceAccountToken: new(bool),
			}

			clusterRole = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:system:etcd-druid",
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{corev1.GroupName},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list", "watch", "delete", "deletecollection"},
					},
					{
						APIGroups: []string{corev1.GroupName},
						Resources: []string{"secrets", "endpoints"},
						Verbs:     []string{"get", "list", "patch", "update", "watch"},
					},
					{
						APIGroups: []string{corev1.GroupName},
						Resources: []string{"events"},
						Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
					},
					{
						APIGroups: []string{corev1.GroupName},
						Resources: []string{"serviceaccounts"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{rbacv1.GroupName},
						Resources: []string{"roles", "rolebindings"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{corev1.GroupName},
						Resources: []string{"services", "configmaps"},
						Verbs:     []string{"get", "list", "patch", "update", "watch", "create", "delete"},
					},
					{
						APIGroups: []string{appsv1.GroupName},
						Resources: []string{"statefulsets"},
						Verbs:     []string{"get", "list", "patch", "update", "watch", "create", "delete"},
					},
					{
						APIGroups: []string{batchv1.GroupName},
						Resources: []string{"jobs"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{druidcorev1alpha1.GroupName},
						Resources: []string{"etcds", "etcdcopybackupstasks"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{druidcorev1alpha1.GroupName},
						Resources: []string{"etcds/status", "etcds/finalizers", "etcdcopybackupstasks/status", "etcdcopybackupstasks/finalizers"},
						Verbs:     []string{"get", "update", "patch", "create"},
					},
					{
						APIGroups: []string{coordinationv1.GroupName},
						Resources: []string{"leases"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"},
					},
					{
						APIGroups: []string{corev1.GroupName},
						Resources: []string{"persistentvolumeclaims"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						APIGroups: []string{policyv1.GroupName},
						Resources: []string{"poddisruptionbudgets"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
				},
			}

			clusterRoleBinding = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:system:etcd-druid",
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "gardener.cloud:system:etcd-druid",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "etcd-druid",
						Namespace: namespace,
					},
				},
			}

			vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-druid-vpa",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
					},
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName: "etcd-druid",
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100M"),
								},
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							},
						},
					},
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "etcd-druid",
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
					},
				},
			}

			configMapImageVectorOverwrite = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
						"resources.gardener.cloud/garbage-collectable-reference": "true",
					},
				},
				Data: map[string]string{
					"images_overwrite.yaml": *imageVectorOverwriteFull,
				},
				Immutable: ptr.To(true),
			}

			deploymentWithoutImageVectorOverwriteFor = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-druid",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
						"high-availability-config.resources.gardener.cloud/type": "controller",
					},
					Annotations: map[string]string{
						references.AnnotationKey(references.KindSecret, "etcd-druid-webhook"): "etcd-druid-webhook",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To[int32](1),
					RevisionHistoryLimit: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "etcd-druid",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"gardener.cloud/role":                            "etcd-druid",
								"networking.gardener.cloud/to-dns":               "allowed",
								"networking.gardener.cloud/to-runtime-apiserver": "allowed",
							},
							Annotations: map[string]string{
								references.AnnotationKey(references.KindSecret, "etcd-druid-webhook"): "etcd-druid-webhook",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Args: []string{
										"--enable-leader-election=true",
										"--disable-etcd-serviceaccount-automount=true",
										"--etcd-workers=25",
										"--enable-etcd-spec-auto-reconcile=false",
										"--webhook-server-port=10250",
										"--webhook-server-tls-server-cert-dir=/etc/webhook-server-tls",
										"--enable-etcd-components-webhook=true",
										"--etcd-components-webhook-exempt-service-accounts=system:serviceaccount:kube-system:generic-garbage-collector",
										"--enable-backup-compaction=true",
										"--compaction-workers=3",
										"--etcd-events-threshold=1000000",
										"--metrics-scrape-wait-duration=1m0s",
										"--active-deadline-duration=3h0m0s",
									},
									Image:           etcdDruidImage,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Name:            "etcd-druid",
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 8080,
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("50m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											MountPath: "/etc/webhook-server-tls",
											Name:      "webhook-server-tls-cert",
											ReadOnly:  true,
										},
									},
								},
							},
							PriorityClassName:  priorityClassName,
							ServiceAccountName: "etcd-druid",
							Volumes: []corev1.Volume{
								{
									Name: "webhook-server-tls-cert",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  "etcd-druid-webhook",
											DefaultMode: ptr.To[int32](420),
										},
									},
								},
							},
						},
					},
				},
			}

			deploymentWithImageVectorOverwrite = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-druid",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
						"high-availability-config.resources.gardener.cloud/type": "controller",
					},
					Annotations: map[string]string{
						references.AnnotationKey(references.KindConfigMap, configMapName):     configMapName,
						references.AnnotationKey(references.KindSecret, "etcd-druid-webhook"): "etcd-druid-webhook",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To[int32](1),
					RevisionHistoryLimit: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "etcd-druid",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"gardener.cloud/role":                            "etcd-druid",
								"networking.gardener.cloud/to-dns":               "allowed",
								"networking.gardener.cloud/to-runtime-apiserver": "allowed",
							},
							Annotations: map[string]string{
								references.AnnotationKey(references.KindConfigMap, configMapName):     configMapName,
								references.AnnotationKey(references.KindSecret, "etcd-druid-webhook"): "etcd-druid-webhook",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Args: []string{
										"--enable-leader-election=true",
										"--disable-etcd-serviceaccount-automount=true",
										"--etcd-workers=25",
										"--enable-etcd-spec-auto-reconcile=false",
										"--webhook-server-port=10250",
										"--webhook-server-tls-server-cert-dir=/etc/webhook-server-tls",
										"--enable-etcd-components-webhook=true",
										"--etcd-components-webhook-exempt-service-accounts=system:serviceaccount:kube-system:generic-garbage-collector",
										"--enable-backup-compaction=true",
										"--compaction-workers=3",
										"--etcd-events-threshold=1000000",
										"--metrics-scrape-wait-duration=1m0s",
										"--active-deadline-duration=3h0m0s",
									},
									Env: []corev1.EnvVar{
										{
											Name:  "IMAGEVECTOR_OVERWRITE",
											Value: "/imagevector_overwrite/images_overwrite.yaml",
										},
									},
									Image:           etcdDruidImage,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Name:            "etcd-druid",
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 8080,
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("50m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											MountPath: "/etc/webhook-server-tls",
											Name:      "webhook-server-tls-cert",
											ReadOnly:  true,
										},
										{
											MountPath: "/imagevector_overwrite",
											Name:      "imagevector-overwrite",
											ReadOnly:  true,
										},
									},
								},
							},
							PriorityClassName:  priorityClassName,
							ServiceAccountName: "etcd-druid",
							Volumes: []corev1.Volume{
								{
									Name: "webhook-server-tls-cert",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  "etcd-druid-webhook",
											DefaultMode: ptr.To[int32](420),
										},
									},
								},
								{
									Name: "imagevector-overwrite",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMapName,
											},
										},
									},
								},
							},
						},
					},
				},
			}

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-druid",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
						"high-availability-config.resources.gardener.cloud/type": "controller",
					},
					Annotations: map[string]string{
						"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8080}]`,
						"networking.resources.gardener.cloud/from-world-to-ports":                        `[{"protocol":"TCP","port":10250}]`,
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "metrics",
							Port:       8080,
							Protocol:   corev1.ProtocolTCP,
							TargetPort: intstr.FromInt32(8080),
						},
						{
							Name:       "webhooks",
							Port:       443,
							Protocol:   corev1.ProtocolTCP,
							TargetPort: intstr.FromInt32(10250),
						},
					},
					Selector: map[string]string{
						"gardener.cloud/role": "etcd-druid",
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			}

			validatingWebhookConfiguration = &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-druid",
					Namespace: namespace,
					Labels:    map[string]string{"gardener.cloud/role": "etcd-druid"},
				},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{
						Name: "etcdcomponents.webhooks.druid.gardener.cloud",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Name:      "etcd-druid",
								Namespace: namespace,
								Path:      ptr.To[string]("/webhooks/etcdcomponents"),
								Port:      ptr.To[int32](443),
							},
							CABundle: nil,
						},
						FailurePolicy:           ptr.To[admissionregistrationv1.FailurePolicyType](admissionregistrationv1.Fail),
						MatchPolicy:             ptr.To[admissionregistrationv1.MatchPolicyType](admissionregistrationv1.Exact),
						SideEffects:             ptr.To[admissionregistrationv1.SideEffectClass](admissionregistrationv1.SideEffectClassNone),
						TimeoutSeconds:          ptr.To[int32](10),
						AdmissionReviewVersions: []string{"v1", "v1beta1"},
						ObjectSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/managed-by": "etcd-druid"}},
						Rules: []admissionregistrationv1.RuleWithOperations{
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{corev1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"serviceaccounts", "services", "configmaps"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{corev1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"persistentvolumeclaims"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{rbacv1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"roles", "rolebindings"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{appsv1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"statefulsets"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{policyv1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"poddisruptionbudgets"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{batchv1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"jobs"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
							},
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{coordinationv1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"leases"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
							},
						},
					},
					{
						Name: "stsscale.etcdcomponents.webhooks.druid.gardener.cloud",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Name:      "etcd-druid",
								Namespace: namespace,
								Path:      ptr.To[string]("/webhooks/etcdcomponents"),
								Port:      ptr.To[int32](443),
							},
							CABundle: nil,
						},
						FailurePolicy:           ptr.To[admissionregistrationv1.FailurePolicyType](admissionregistrationv1.Fail),
						MatchPolicy:             ptr.To[admissionregistrationv1.MatchPolicyType](admissionregistrationv1.Exact),
						SideEffects:             ptr.To[admissionregistrationv1.SideEffectClass](admissionregistrationv1.SideEffectClassNone),
						TimeoutSeconds:          ptr.To[int32](10),
						AdmissionReviewVersions: []string{"v1", "v1beta1"},
						Rules: []admissionregistrationv1.RuleWithOperations{
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{appsv1.GroupName},
									APIVersions: []string{"v1"},
									Resources:   []string{"statefulsets/scale"},
									Scope:       ptr.To[admissionregistrationv1.ScopeType](admissionregistrationv1.AllScopes),
								},
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
							},
						},
					},
				},
			}

			podDisruption = &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-druid",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "etcd-druid",
					},
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MaxUnavailable: ptr.To(intstr.FromInt32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "etcd-druid",
						},
					},
					UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
				},
				Status: policyv1.PodDisruptionBudgetStatus{
					CurrentHealthy:     0,
					DesiredHealthy:     0,
					DisruptionsAllowed: 0,
					ExpectedPods:       0,
				},
			}

			serviceMonitor = &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cache-etcd-druid",
					Namespace: namespace,
					Labels: map[string]string{
						"prometheus": "cache",
					},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: "metrics",
							MetricRelabelConfigs: []monitoringv1.RelabelConfig{
								{
									Action: "keep",
									Regex:  "^(etcddruid_compaction_jobs_total|etcddruid_compaction_jobs_current|etcddruid_compaction_job_duration_seconds_bucket|etcddruid_compaction_job_duration_seconds_sum|etcddruid_compaction_job_duration_seconds_count|etcddruid_compaction_num_delta_events)$",
									SourceLabels: []monitoringv1.LabelName{
										"__name__",
									},
								},
							},
						},
					},
					NamespaceSelector: monitoringv1.NamespaceSelector{},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "etcd-druid",
						},
					},
				},
			}
		)

		BeforeEach(func() {
			imageVectorOverwrite = nil
			featureGates = nil
		})

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			Expect(bootstrapper.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			expectedResources = []client.Object{
				serviceAccount,
				clusterRole,
				clusterRoleBinding,
				podDisruption,
				vpa,
				service,
				validatingWebhookConfiguration,
				serviceMonitor,
			}

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			manifests, err := test.BrotliDecompression(managedResourceSecret.Data["data.yaml.br"])
			Expect(err).NotTo(HaveOccurred())
			Expect(manifests).ToNot(BeEmpty())
		})

		AfterEach(func() {
			Expect(managedResource).To(consistOf(expectedResources...))
		})

		Context("w/o image vector overwrite", func() {
			It("should successfully deploy all the resources (w/o image vector overwrite)", func() {
				expectedResources = append(expectedResources, deploymentWithoutImageVectorOverwriteFor)
			})
		})

		Context("w/ image vector overwrite", func() {
			BeforeEach(func() {
				imageVectorOverwrite = imageVectorOverwriteFull
			})

			It("should successfully deploy all the resources (w/ image vector overwrite)", func() {
				bootstrapper = NewBootstrapper(c, namespace, etcdConfig, etcdDruidImage, imageVectorOverwriteFull, sm, secretNameCA, priorityClassName)

				expectedResources = append(expectedResources,
					deploymentWithImageVectorOverwrite,
					configMapImageVectorOverwrite,
				)
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(bootstrapper.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime ManagedResource is unhealthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the runtime ManagedResource is still progressing", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("still progressing")))
			})

			It("should succeed because the ManagedResource is healthy and progressed", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(bootstrapper.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(bootstrapper.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(bootstrapper.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
