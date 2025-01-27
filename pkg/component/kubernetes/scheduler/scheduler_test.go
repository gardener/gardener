// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package scheduler_test

import (
	"context"
	"os"
	"strconv"

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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetes/scheduler"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("KubeScheduler", func() {
	var (
		c                             client.Client
		sm                            secretsmanager.Interface
		kubeScheduler                 component.DeployWaiter
		ctx                           = context.Background()
		namespace                     = "shoot--foo--bar"
		runtimeVersion, targetVersion *semver.Version
		image                               = "registry.k8s.io/kube-scheduler:v1.27.2"
		replicas                      int32 = 1
		profileBinPacking                   = gardencorev1beta1.SchedulingProfileBinPacking
		configEmpty                   *gardencorev1beta1.KubeSchedulerConfig
		configFull                    = &gardencorev1beta1.KubeSchedulerConfig{
			KubernetesConfig: gardencorev1beta1.KubernetesConfig{
				FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false},
			},
			KubeMaxPDVols: ptr.To("23"),
			Profile:       &profileBinPacking,
		}
		consistOf func(...client.Object) types.GomegaMatcher

		secretNameClientCA = "ca-client"
		secretNameServer   = "kube-scheduler-server"

		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		vpaName                          = "kube-scheduler-vpa"
		pdbName                          = "kube-scheduler"
		prometheusRuleName               = "shoot-kube-scheduler"
		serviceMonitorName               = "shoot-kube-scheduler"
		serviceName                      = "kube-scheduler"
		secretName                       = "shoot-access-kube-scheduler"
		deploymentName                   = "kube-scheduler"
		managedResourceName              = "shoot-core-kube-scheduler"
		managedResourceSecretName        = "managedresource-shoot-core-kube-scheduler"

		configMapFor = func(componentConfigFilePath string) *corev1.ConfigMap {
			data, err := os.ReadFile(componentConfigFilePath)
			Expect(err).NotTo(HaveOccurred())
			componentConfigYAML := string(data)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "kube-scheduler-config",
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Data: map[string]string{"config.yaml": componentConfigYAML},
			}
			Expect(kubernetesutils.MakeUnique(cm)).To(Succeed())
			return cm
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "kube-scheduler",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				ResourceVersion: "0",
			},
			Type: corev1.SecretTypeOpaque,
		}

		pdbMaxUnavailable = intstr.FromInt32(1)
		pdbFor            = func(runtimeVersion *semver.Version) *policyv1.PodDisruptionBudget {
			pdb := &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pdbName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":  "kubernetes",
						"role": "scheduler",
					},
					ResourceVersion: "1",
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MaxUnavailable: &pdbMaxUnavailable,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "scheduler",
						},
					},
				},
			}

			unhealthyPodEvictionPolicyAlwatysAllow := policyv1.AlwaysAllow
			if versionutils.ConstraintK8sGreaterEqual126.Check(runtimeVersion) {
				pdb.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwatysAllow
			}

			return pdb
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace, ResourceVersion: "1"},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					}},
				},
			},
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "scheduler",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":10259}]`,
				},
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "scheduler",
				},
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Protocol: corev1.ProtocolTCP,
						Port:     10259,
					},
				},
			},
		}
		deploymentFor = func(config *gardencorev1beta1.KubeSchedulerConfig, componentConfigFilePath string) *appsv1.Deployment {
			var env []corev1.EnvVar
			if config != nil && config.KubeMaxPDVols != nil {
				env = append(env, corev1.EnvVar{
					Name:  "KUBE_MAX_PD_VOLS",
					Value: *config.KubeMaxPDVols,
				})
			}

			configMap := configMapFor(componentConfigFilePath)

			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "scheduler",
						"gardener.cloud/role": "controlplane",
						"high-availability-config.resources.gardener.cloud/type":             "controller",
						"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
					},
					ResourceVersion: "1",
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: ptr.To[int32](1),
					Replicas:             &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "scheduler",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                                "kubernetes",
								"role":                               "scheduler",
								"gardener.cloud/role":                "controlplane",
								"maintenance.gardener.cloud/restart": "true",
								"networking.gardener.cloud/to-dns":   "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							AutomountServiceAccountToken: ptr.To(false),
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: ptr.To(true),
								RunAsUser:    ptr.To[int64](65532),
								RunAsGroup:   ptr.To[int64](65532),
								FSGroup:      ptr.To[int64](65532),
							},
							Containers: []corev1.Container{
								{
									Name:            "kube-scheduler",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         commandForKubernetesVersion(10259, featureGateFlags(config)...),
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
									},
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/healthz",
												Scheme: corev1.URISchemeHTTPS,
												Port:   intstr.FromInt32(10259),
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
											Name:          "metrics",
											ContainerPort: 10259,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									Env: env,
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("5m"),
											corev1.ResourceMemory: resource.MustParse("30M"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "client-ca",
											MountPath: "/var/lib/kube-scheduler-client-ca",
										},
										{
											Name:      "kube-scheduler-server",
											MountPath: "/var/lib/kube-scheduler-server",
										},
										{
											Name:      "kube-scheduler-config",
											MountPath: "/var/lib/kube-scheduler-config",
										},
									},
								},
							},
							PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane300,
							Volumes: []corev1.Volume{
								{
									Name: "client-ca",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											DefaultMode: ptr.To[int32](420),
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretNameClientCA,
														},
														Items: []corev1.KeyToPath{{
															Key:  "bundle.crt",
															Path: "bundle.crt",
														}},
													},
												},
											},
										},
									},
								},
								{
									Name: "kube-scheduler-server",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  secretNameServer,
											DefaultMode: ptr.To[int32](0640),
										},
									},
								},
								{
									Name: "kube-scheduler-config",
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

			Expect(gardenerutils.InjectGenericKubeconfig(deploy, genericTokenKubeconfigSecretName, secret.Name)).To(Succeed())
			Expect(references.InjectAnnotations(deploy)).To(Succeed())
			return deploy
		}

		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-kube-scheduler",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "kube-scheduler.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "KubeSchedulerDown",
							Expr:  intstr.FromString(`absent(up{job="kube-scheduler"} == 1)`),
							For:   ptr.To(monitoringv1.Duration("15m")),
							Labels: map[string]string{
								"service":    "kube-scheduler",
								"severity":   "critical",
								"type":       "seed",
								"visibility": "all",
							},
							Annotations: map[string]string{
								"summary":     "Kube Scheduler is down.",
								"description": "New pods are not being assigned to nodes.",
							},
						},
						{
							Record: "cluster:scheduler_e2e_scheduling_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.99, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.99"},
						},
						{
							Record: "cluster:scheduler_e2e_scheduling_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.9, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.9"},
						},
						{
							Record: "cluster:scheduler_e2e_scheduling_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.5, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.5"},
						},
						{
							Record: "cluster:scheduler_scheduling_algorithm_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.99, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.99"},
						},
						{
							Record: "cluster:scheduler_scheduling_algorithm_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.9, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.9"},
						},
						{
							Record: "cluster:scheduler_scheduling_algorithm_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.5, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.5"},
						},
						{
							Record: "cluster:scheduler_binding_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.99, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.99"},
						},
						{
							Record: "cluster:scheduler_binding_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.9, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.9"},
						},
						{
							Record: "cluster:scheduler_binding_duration_seconds:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.5, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))`),
							Labels: map[string]string{"quantile": "0.5"},
						},
					},
				}},
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-kube-scheduler",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "kubernetes",
					"role": "scheduler",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port:      "metrics",
					Scheme:    "https",
					TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}},
					Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
						Key:                  "token",
					}},
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					}},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(scheduler_binding_duration_seconds_bucket|scheduler_e2e_scheduling_duration_seconds_bucket|scheduler_scheduling_algorithm_duration_seconds_bucket|rest_client_requests_total|process_max_fds|process_open_fds)$`,
					}},
				}},
			},
		}

		clusterRoleBinding1 = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:kube-scheduler",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:kube-scheduler",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "kube-scheduler",
					Namespace: "kube-system",
				},
			},
		}
		clusterRoleBinding2 = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:kube-scheduler-volume",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:volume-scheduler",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "kube-scheduler",
					Namespace: "kube-system",
				},
			},
		}

		managedResource *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		targetVersion = semver.MustParse("1.27.2")
		runtimeVersion = semver.MustParse("1.25.2")
		kubeScheduler = New(c, namespace, sm, runtimeVersion, targetVersion, image, replicas, configEmpty)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
					Labels:    map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{
						{Name: managedResourceSecretName},
					},
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  ptr.To(false),
				},
			}
		})

		DescribeTable("success tests for various kubernetes versions",
			func(targetVersion, runtimeVersion string, config *gardencorev1beta1.KubeSchedulerConfig, expectedComponentConfigFilePath string) {
				targetSemverVersion, err := semver.NewVersion(targetVersion)
				Expect(err).NotTo(HaveOccurred())

				runtimeSemverVersion, err := semver.NewVersion(runtimeVersion)
				Expect(err).NotTo(HaveOccurred())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

				kubeScheduler = New(c, namespace, sm, runtimeSemverVersion, targetSemverVersion, image, replicas, config)
				Expect(kubeScheduler.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels:          map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))
				Expect(managedResource).To(consistOf(clusterRoleBinding1, clusterRoleBinding2))

				expectedConfigMap := configMapFor(expectedComponentConfigFilePath)
				actualConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedConfigMap.Name,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualConfigMap), actualConfigMap)).To(Succeed())
				Expect(actualConfigMap).To(DeepEqual(expectedConfigMap))

				actualDeployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deploymentName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualDeployment), actualDeployment)).To(Succeed())
				Expect(actualDeployment).To(DeepEqual(deploymentFor(config, expectedComponentConfigFilePath)))

				actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualVPA), actualVPA)).To(Succeed())
				Expect(actualVPA).To(DeepEqual(vpa))

				actualService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualService), actualService)).To(Succeed())
				Expect(actualService).To(DeepEqual(service))

				actualPDB := &policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pdbName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualPDB), actualPDB)).To(Succeed())
				Expect(actualPDB).To(DeepEqual(pdbFor(runtimeSemverVersion)))

				actualPrometheusRule := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      prometheusRuleName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualPrometheusRule), actualPrometheusRule)).To(Succeed())
				Expect(actualPrometheusRule).To(DeepEqual(prometheusRule))

				componenttest.PrometheusRule(prometheusRule, "testdata/shoot-kube-scheduler.prometheusrule.test.yaml")

				actualServiceMonitor := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceMonitorName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualServiceMonitor), actualServiceMonitor)).To(Succeed())
				Expect(actualServiceMonitor).To(DeepEqual(serviceMonitor))
			},

			Entry("kubernetes 1.25 w/o config", "1.25.0", "1.25.0", configEmpty, "testdata/component-config-1.25.yaml"),
			Entry("kubernetes 1.25 w/ full config", "1.25.0", "1.25.0", configFull, "testdata/component-config-1.25-bin-packing.yaml"),
			Entry("kubernetes 1.26 w/o config", "1.26.0", "1.26.0", configEmpty, "testdata/component-config-1.25.yaml"),
			Entry("kubernetes 1.26 w/ full config", "1.26.0", "1.26.0", configFull, "testdata/component-config-1.25-bin-packing.yaml"),
		)
	})

	Describe("#Destroy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeScheduler.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeScheduler.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeScheduler.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

func commandForKubernetesVersion(port int32, featureGateFlags ...string) []string {
	var command []string

	command = append(command,
		"/usr/local/bin/kube-scheduler",
		"--config=/var/lib/kube-scheduler-config/config.yaml",
		"--authentication-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--authorization-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--client-ca-file=/var/lib/kube-scheduler-client-ca/bundle.crt",
		"--tls-cert-file=/var/lib/kube-scheduler-server/tls.crt",
		"--tls-private-key-file=/var/lib/kube-scheduler-server/tls.key",
		"--secure-port="+strconv.Itoa(int(port)),
	)

	command = append(command, featureGateFlags...)
	command = append(command, "--v=2")

	return command
}

func featureGateFlags(config *gardencorev1beta1.KubeSchedulerConfig) []string {
	var out []string

	if config != nil && config.FeatureGates != nil {
		out = append(out, kubernetesutils.FeatureGatesToCommandLineParameter(config.FeatureGates))
	}

	return out
}
