// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"context"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
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
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Etcd", func() {
	var (
		c            client.Client
		bootstrapper component.DeployWaiter
		etcdConfig   *config.ETCDConfig
		consistOf    func(...client.Object) types.GomegaMatcher

		ctx                      = context.Background()
		namespace                = "shoot--foo--bar"
		kubernetesVersion        *semver.Version
		etcdDruidImage           = "etcd/druid:1.2.3"
		imageVectorOverwrite     *string
		imageVectorOverwriteFull = ptr.To("some overwrite")

		priorityClassName = "some-priority-class"

		featureGates map[string]bool

		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource

		managedResourceName       = "etcd-druid"
		managedResourceSecretName = "managedresource-" + managedResourceName
	)

	JustBeforeEach(func() {
		etcdConfig = &config.ETCDConfig{
			ETCDController: &config.ETCDController{
				Workers: ptr.To[int64](25),
			},
			CustodianController: &config.CustodianController{
				Workers: ptr.To[int64](3),
			},
			BackupCompactionController: &config.BackupCompactionController{
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

		bootstrapper = NewBootstrapper(c, namespace, kubernetesVersion, etcdConfig, etcdDruidImage, imageVectorOverwrite, priorityClassName)

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
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list", "watch", "delete", "deletecollection"},
					},
					{
						APIGroups: []string{""},
						Resources: []string{"secrets", "endpoints"},
						Verbs:     []string{"get", "list", "patch", "update", "watch"},
					},
					{
						APIGroups: []string{""},
						Resources: []string{"events"},
						Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
					},
					{
						APIGroups: []string{""},
						Resources: []string{"serviceaccounts"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{"rbac.authorization.k8s.io"},
						Resources: []string{"roles", "rolebindings"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{""},
						Resources: []string{"services", "configmaps"},
						Verbs:     []string{"get", "list", "patch", "update", "watch", "create", "delete"},
					},
					{
						APIGroups: []string{"apps"},
						Resources: []string{"statefulsets"},
						Verbs:     []string{"get", "list", "patch", "update", "watch", "create", "delete"},
					},
					{
						APIGroups: []string{"batch"},
						Resources: []string{"jobs"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{"druid.gardener.cloud"},
						Resources: []string{"etcds", "etcdcopybackupstasks"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
					{
						APIGroups: []string{"druid.gardener.cloud"},
						Resources: []string{"etcds/status", "etcds/finalizers", "etcdcopybackupstasks/status", "etcdcopybackupstasks/finalizers"},
						Verbs:     []string{"get", "update", "patch", "create"},
					},
					{
						APIGroups: []string{"coordination.k8s.io"},
						Resources: []string{"leases"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"},
					},
					{
						APIGroups: []string{""},
						Resources: []string{"persistentvolumeclaims"},
						Verbs:     []string{"get", "list", "watch"},
					},
					{
						APIGroups: []string{"policy"},
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
								ContainerName: "*",
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100M"),
								},
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

			deploymentWithoutImageVectorOverwriteFor = func(useEtcdWrapper bool) *appsv1.Deployment {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "etcd-druid",
						Namespace: namespace,
						Labels: map[string]string{
							"gardener.cloud/role": "etcd-druid",
							"high-availability-config.resources.gardener.cloud/type": "controller",
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
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Args: []string{
											"--enable-leader-election=true",
											"--ignore-operation-annotation=false",
											"--disable-etcd-serviceaccount-automount=true",
											"--workers=25",
											"--custodian-workers=3",
											"--compaction-workers=3",
											"--enable-backup-compaction=true",
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
											Limits: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("512Mi"),
											},
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("50m"),
												corev1.ResourceMemory: resource.MustParse("128Mi"),
											},
										},
									},
								},
								PriorityClassName:  priorityClassName,
								ServiceAccountName: "etcd-druid",
							},
						},
					},
				}

				// Add feature gate command if useEtcdWrapper is true
				if useEtcdWrapper {
					deployment.Spec.Template.Spec.Containers[0].Args = append(
						deployment.Spec.Template.Spec.Containers[0].Args,
						"--feature-gates=UseEtcdWrapper=true",
					)
				}

				return deployment
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
						references.AnnotationKey(references.KindConfigMap, configMapName): configMapName,
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
								references.AnnotationKey(references.KindConfigMap, configMapName): configMapName,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Args: []string{
										"--enable-leader-election=true",
										"--ignore-operation-annotation=false",
										"--disable-etcd-serviceaccount-automount=true",
										"--workers=25",
										"--custodian-workers=3",
										"--compaction-workers=3",
										"--enable-backup-compaction=true",
										"--etcd-events-threshold=1000000",
										"--metrics-scrape-wait-duration=1m0s",
										"--active-deadline-duration=3h0m0s",
									},
									Env: []corev1.EnvVar{
										{
											Name:  "IMAGEVECTOR_OVERWRITE",
											Value: "/charts_overwrite/images_overwrite.yaml",
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
										Limits: corev1.ResourceList{
											corev1.ResourceMemory: resource.MustParse("512Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("50m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											MountPath: "/charts_overwrite",
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
					},
					Selector: map[string]string{
						"gardener.cloud/role": "etcd-druid",
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			}

			podDisruptionFor = func(k8sGreaterEquals126 bool) *policyv1.PodDisruptionBudget {
				podDisruptionBudget := &policyv1.PodDisruptionBudget{
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
					},
					Status: policyv1.PodDisruptionBudgetStatus{
						CurrentHealthy:     0,
						DesiredHealthy:     0,
						DisruptionsAllowed: 0,
						ExpectedPods:       0,
					},
				}

				if k8sGreaterEquals126 {
					podDisruptionBudget.Spec.UnhealthyPodEvictionPolicy = ptr.To(policyv1.AlwaysAllow)
				}

				return podDisruptionBudget
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
			kubernetesVersion = semver.MustParse("1.25.0")
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
				vpa,
				service,
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
			JustBeforeEach(func() {
				expectedResources = append(expectedResources, deploymentWithoutImageVectorOverwriteFor(false))
			})

			Context("kubernetes versions < 1.26", func() {
				It("should successfully deploy all the resources (w/o image vector overwrite)", func() {
					expectedResources = append(expectedResources, podDisruptionFor(false))
				})
			})

			Context("kubernetes versions >= 1.26", func() {
				BeforeEach(func() {
					kubernetesVersion = semver.MustParse("1.26.0")
				})

				It("should successfully deploy all the resources", func() {
					expectedResources = append(expectedResources, podDisruptionFor(true))
				})
			})
		})

		Context("w/ image vector overwrite", func() {
			BeforeEach(func() {
				imageVectorOverwrite = imageVectorOverwriteFull
			})

			It("should successfully deploy all the resources (w/ image vector overwrite)", func() {
				bootstrapper = NewBootstrapper(c, namespace, kubernetesVersion, etcdConfig, etcdDruidImage, imageVectorOverwriteFull, priorityClassName)

				expectedResources = append(expectedResources,
					deploymentWithImageVectorOverwrite,
					configMapImageVectorOverwrite,
					podDisruptionFor(false),
				)
			})
		})

		Context("w/ feature gates being present in etcd config", func() {
			BeforeEach(func() {
				featureGates = map[string]bool{
					"UseEtcdWrapper": true,
				}
			})

			It("should successfully deploy all the resources", func() {
				expectedResources = append(expectedResources,
					deploymentWithoutImageVectorOverwriteFor(true),
					podDisruptionFor(false),
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
