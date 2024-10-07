// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
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
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("KubeControllerManager", func() {
	var (
		ctx        = context.TODO()
		testLogger = logr.Discard()

		c             client.Client
		fakeInterface kubernetes.Interface

		sm                    secretsmanager.Interface
		kubeControllerManager Interface
		values                Values

		_, podCIDR1, _           = net.ParseCIDR("100.96.0.0/11")
		_, podCIDR2, _           = net.ParseCIDR("2001:db8::/64")
		podCIDRs                 = []net.IPNet{*podCIDR1, *podCIDR2}
		_, serviceCIDR1, _       = net.ParseCIDR("100.64.0.0/13")
		_, serviceCIDR2, _       = net.ParseCIDR("2001:db8::/64")
		serviceCIDRs             = []net.IPNet{*serviceCIDR1, *serviceCIDR2}
		namespace                = "shoot--foo--bar"
		version                  = "1.27.3"
		semverVersion, _         = semver.NewVersion(version)
		runtimeKubernetesVersion = semver.MustParse("1.31.1")
		image                    = "registry.k8s.io/kube-controller-manager:v1.31.1"
		isWorkerless             = false
		priorityClassName        = v1beta1constants.PriorityClassNameShootControlPlane300

		hpaConfig = gardencorev1beta1.HorizontalPodAutoscalerConfig{
			CPUInitializationPeriod: &metav1.Duration{Duration: 5 * time.Minute},
			DownscaleStabilization:  &metav1.Duration{Duration: 5 * time.Minute},
			InitialReadinessDelay:   &metav1.Duration{Duration: 30 * time.Second},
			SyncPeriod:              &metav1.Duration{Duration: 30 * time.Second},
			Tolerance:               ptr.To(float64(0.1)),
		}

		nodeCIDRMask           int32 = 24
		podEvictionTimeout           = metav1.Duration{Duration: 3 * time.Minute}
		nodeMonitorGracePeriod       = metav1.Duration{Duration: 3 * time.Minute}
		kcmConfig                    = gardencorev1beta1.KubeControllerManagerConfig{
			KubernetesConfig:              gardencorev1beta1.KubernetesConfig{},
			HorizontalPodAutoscalerConfig: &hpaConfig,
			NodeCIDRMaskSize:              &nodeCIDRMask,
			PodEvictionTimeout:            &podEvictionTimeout,
			NodeMonitorGracePeriod:        &nodeMonitorGracePeriod,
		}
		clusterSigningDuration = ptr.To(time.Hour)
		controllerWorkers      = ControllerWorkers{
			StatefulSet:         ptr.To(1),
			Deployment:          ptr.To(2),
			ReplicaSet:          ptr.To(3),
			Endpoint:            ptr.To(4),
			GarbageCollector:    ptr.To(5),
			Namespace:           ptr.To(6),
			ResourceQuota:       ptr.To(7),
			ServiceEndpoint:     ptr.To(8),
			ServiceAccountToken: ptr.To(9),
		}
		controllerWorkersWithDisabledControllers = ControllerWorkers{
			StatefulSet:         ptr.To(1),
			Deployment:          ptr.To(2),
			ReplicaSet:          ptr.To(3),
			Endpoint:            ptr.To(4),
			GarbageCollector:    ptr.To(5),
			Namespace:           ptr.To(0),
			ResourceQuota:       ptr.To(0),
			ServiceEndpoint:     ptr.To(8),
			ServiceAccountToken: ptr.To(0),
		}
		controllerSyncPeriods = ControllerSyncPeriods{
			ResourceQuota: ptr.To(time.Minute),
		}

		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		vpaName                          = "kube-controller-manager-vpa"
		pdbName                          = "kube-controller-manager"
		secretName                       = "shoot-access-kube-controller-manager"
		serviceName                      = "kube-controller-manager"
		managedResourceName              = "shoot-core-kube-controller-manager"
		managedResourceSecretName        = "managedresource-shoot-core-kube-controller-manager"

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "kube-controller-manager",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				ResourceVersion: "1",
			},
			Type: corev1.SecretTypeOpaque,
		}

		pdb = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pdbName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "controller-manager",
				},
				ResourceVersion: "1",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "kubernetes",
						"role": "controller-manager",
					},
				},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}

		vpaFor = func(isScaleDownDisabled bool) *vpaautoscalingv1.VerticalPodAutoscaler {
			vpa := &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace, ResourceVersion: "1"},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "kube-controller-manager",
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
							ContainerName:    "kube-controller-manager",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						}},
					},
				},
			}

			if isScaleDownDisabled {
				vpa.Labels = map[string]string{"autoscaling.gardener.cloud/eviction-requirements": "managed-by-controller"}
				vpa.Annotations = map[string]string{"eviction-requirements.autoscaling.gardener.cloud/downscale-restriction": "never"}
			}

			return vpa
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "controller-manager",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":10257}]`,
				},
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "controller-manager",
				},
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Protocol: corev1.ProtocolTCP,
						Port:     10257,
					},
				},
			},
		}

		serviceMonitor = func(prometheusName, namePrefix string) *monitoringv1.ServiceMonitor {
			return &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:            prometheusName + "-" + namePrefix + "kube-controller-manager",
					Namespace:       namespace,
					Labels:          map[string]string{"prometheus": prometheusName},
					ResourceVersion: "1",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "kubernetes", "role": "controller-manager"}},
					Endpoints: []monitoringv1.Endpoint{{
						Port:      "metrics",
						Scheme:    "https",
						TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}},
						Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-" + prometheusName},
							Key:                  "token",
						}},
						RelabelConfigs: []monitoringv1.RelabelConfig{{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						}},
						MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(rest_client_requests_total|process_max_fds|process_open_fds)$`,
						}},
					}},
				},
			}
		}
		prometheusRule = func(prometheusName, namePrefix string) *monitoringv1.PrometheusRule {
			labels := map[string]string{
				"service":    v1beta1constants.DeploymentNameKubeControllerManager,
				"severity":   "critical",
				"visibility": "all",
			}

			if namePrefix != "" {
				labels["topology"] = "garden"
			} else {
				labels["type"] = "seed"
			}

			return &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:            prometheusName + "-" + namePrefix + "kube-controller-manager",
					Namespace:       namespace,
					Labels:          map[string]string{"prometheus": prometheusName},
					ResourceVersion: "1",
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{{
						Name: "kube-controller-manager.rules",
						Rules: []monitoringv1.Rule{{
							Alert:  "KubeControllerManagerDown",
							Expr:   intstr.FromString(`absent(up{job="` + namePrefix + `kube-controller-manager"} == 1)`),
							For:    ptr.To(monitoringv1.Duration("15m")),
							Labels: labels,
							Annotations: map[string]string{
								"summary":     "Kube Controller Manager is down.",
								"description": "Deployments and replication controllers are not making progress.",
							},
						}},
					}},
				},
			}
		}

		replicas      int32 = 1
		deploymentFor       = func(config *gardencorev1beta1.KubeControllerManagerConfig, isWorkerless bool, controllerWorkers ControllerWorkers) *appsv1.Deployment {
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DeploymentNameKubeControllerManager,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "controller-manager",
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
							"role": "controller-manager",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                                "kubernetes",
								"role":                               "controller-manager",
								"gardener.cloud/role":                "controlplane",
								"maintenance.gardener.cloud/restart": "true",
								"networking.gardener.cloud/to-dns":   "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							AutomountServiceAccountToken: ptr.To(false),
							PriorityClassName:            priorityClassName,
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: ptr.To(true),
								RunAsUser:    ptr.To[int64](65532),
								RunAsGroup:   ptr.To[int64](65532),
								FSGroup:      ptr.To[int64](65532),
							},
							Containers: []corev1.Container{
								{
									Name:            "kube-controller-manager",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command: commandForKubernetesVersion(
										10257,
										config.NodeCIDRMaskSize,
										config.NodeMonitorGracePeriod,
										namespace,
										isWorkerless,
										serviceCIDRs,
										podCIDRs,
										getHorizontalPodAutoscalerConfig(config.HorizontalPodAutoscalerConfig),
										kubernetesutils.FeatureGatesToCommandLineParameter(config.FeatureGates),
										clusterSigningDuration,
										controllerWorkers,
										controllerSyncPeriods,
									),
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/healthz",
												Scheme: corev1.URISchemeHTTPS,
												Port:   intstr.FromInt32(10257),
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
											ContainerPort: 10257,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("5m"),
											corev1.ResourceMemory: resource.MustParse("30M"),
										},
									},
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "ca",
											MountPath: "/srv/kubernetes/ca",
										},
										{
											Name:      "ca-client",
											MountPath: "/srv/kubernetes/ca-client",
										},
										{
											Name:      "service-account-key",
											MountPath: "/srv/kubernetes/service-account-key",
										},
										{
											Name:      "server",
											MountPath: "/var/lib/kube-controller-manager-server",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "ca",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "ca",
										},
									},
								},
								{
									Name: "ca-client",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  "ca-client-current",
											DefaultMode: ptr.To[int32](0640),
										},
									},
								},
								{
									Name: "service-account-key",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  "service-account-key-current",
											DefaultMode: ptr.To[int32](0640),
										},
									},
								},
								{
									Name: "server",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  "kube-controller-manager-server",
											DefaultMode: ptr.To[int32](0640),
										},
									},
								},
							},
						},
					},
				},
			}

			if !isWorkerless {
				deploy.Spec.Template.Spec.Containers[0].VolumeMounts = append(deploy.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "ca-kubelet",
					MountPath: "/srv/kubernetes/ca-kubelet",
				})

				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, corev1.Volume{
					Name: "ca-kubelet",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  "ca-kubelet-current",
							DefaultMode: ptr.To[int32](0640),
						},
					},
				})
			}

			Expect(gardenerutils.InjectGenericKubeconfig(deploy, genericTokenKubeconfigSecretName, secret.Name)).To(Succeed())
			return deploy
		}

		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
creationTimestamp: null
name: gardener.cloud:target:kube-controller-manager
roleRef:
apiGroup: rbac.authorization.k8s.io
kind: ClusterRole
name: system:kube-controller-manager
subjects:
- kind: ServiceAccount
name: kube-controller-manager
namespace: kube-system
`
		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource

		emptyConfig                = &gardencorev1beta1.KubeControllerManagerConfig{}
		configWithAutoscalerConfig = &gardencorev1beta1.KubeControllerManagerConfig{
			// non default configuration
			HorizontalPodAutoscalerConfig: &gardencorev1beta1.HorizontalPodAutoscalerConfig{
				CPUInitializationPeriod: &metav1.Duration{Duration: 10 * time.Minute},
				DownscaleStabilization:  &metav1.Duration{Duration: 10 * time.Minute},
				InitialReadinessDelay:   &metav1.Duration{Duration: 20 * time.Second},
				SyncPeriod:              &metav1.Duration{Duration: 20 * time.Second},
				Tolerance:               ptr.To(float64(0.3)),
			},
			NodeCIDRMaskSize: nil,
		}
		configWithFeatureFlags           = &gardencorev1beta1.KubeControllerManagerConfig{KubernetesConfig: gardencorev1beta1.KubernetesConfig{FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false}}}
		configWithNodeCIDRMaskSize       = &gardencorev1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: ptr.To[int32](26)}
		configWithPodEvictionTimeout     = &gardencorev1beta1.KubeControllerManagerConfig{PodEvictionTimeout: &podEvictionTimeout}
		configWithNodeMonitorGracePeriod = &gardencorev1beta1.KubeControllerManagerConfig{NodeMonitorGracePeriod: &nodeMonitorGracePeriod}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeInterface = kubernetesfake.NewClientSetBuilder().WithAPIReader(c).WithClient(c).Build()
		sm = fakesecretsmanager.New(c, namespace)

		values = Values{
			RuntimeVersion:        runtimeKubernetesVersion,
			TargetVersion:         semverVersion,
			Image:                 image,
			Config:                &kcmConfig,
			PriorityClassName:     priorityClassName,
			IsWorkerless:          isWorkerless,
			PodNetworks:           podCIDRs,
			ServiceNetworks:       serviceCIDRs,
			ManagedResourceLabels: map[string]string{"foo": "bar"},
		}
		kubeControllerManager = New(
			testLogger,
			fakeInterface,
			namespace,
			sm,
			values,
		)

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clusterrolebinding____gardener.cloud_target_kube-controller-manager.yaml": []byte(clusterRoleBindingYAML),
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
				Labels: map[string]string{
					"origin": "gardener",
					"foo":    "bar",
				},
				ResourceVersion: "1",
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: managedResourceSecretName},
				},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  ptr.To(true),
			},
		}

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client-current", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-kubelet-current", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "service-account-key-current", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		verifyDeployment := func(config *gardencorev1beta1.KubeControllerManagerConfig, isScaleDownDisabled bool, controllerWorkers ControllerWorkers) {
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
					KeepObjects: ptr.To(true),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			actualDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(actualDeployment), actualDeployment)).To(Succeed())
			Expect(actualDeployment).To(Equal(deploymentFor(config, isWorkerless, controllerWorkers)))

			actualService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(actualService), actualService)).To(Succeed())
			Expect(actualService).To(DeepEqual(service))

			actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(actualVPA), actualVPA)).To(Succeed())
			Expect(actualVPA).To(DeepEqual(vpaFor(isScaleDownDisabled)))

			actualPDB := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: pdbName, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(actualPDB), actualPDB)).To(Succeed())
			Expect(actualPDB).To(DeepEqual(pdb))

			expectedServiceMonitor := serviceMonitor("shoot", "")
			actualServiceMonitor := &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: expectedServiceMonitor.Name, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(actualServiceMonitor), actualServiceMonitor)).To(Succeed())
			Expect(actualServiceMonitor).To(DeepEqual(expectedServiceMonitor))

			expectedPrometheusRule := prometheusRule("shoot", "")
			actualPrometheusRule := &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: expectedPrometheusRule.Name, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(actualPrometheusRule), actualPrometheusRule)).To(Succeed())
			Expect(actualPrometheusRule).To(DeepEqual(expectedPrometheusRule))

			componenttest.PrometheusRule(actualPrometheusRule, "testdata/shoot-kube-controller-manager.prometheusrule.test.yaml")

			actualSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(actualSecret), actualSecret)).To(Succeed())
			Expect(actualSecret).To(DeepEqual(secret))
		}

		DescribeTable("success tests for various kubernetes versions (shoots with workers)",
			func(config *gardencorev1beta1.KubeControllerManagerConfig, isScaleDownDisabled bool, runtimeKubernetesVersion *semver.Version) {
				isWorkerless = false
				semverVersion, err := semver.NewVersion(version)
				Expect(err).NotTo(HaveOccurred())

				values = Values{
					RuntimeVersion:         runtimeKubernetesVersion,
					TargetVersion:          semverVersion,
					Image:                  image,
					Config:                 config,
					PriorityClassName:      priorityClassName,
					IsScaleDownDisabled:    isScaleDownDisabled,
					IsWorkerless:           isWorkerless,
					PodNetworks:            podCIDRs,
					ServiceNetworks:        serviceCIDRs,
					ClusterSigningDuration: clusterSigningDuration,
					ControllerWorkers:      controllerWorkers,
					ControllerSyncPeriods:  controllerSyncPeriods,
				}
				kubeControllerManager = New(testLogger, fakeInterface, namespace, sm, values)
				kubeControllerManager.SetReplicaCount(replicas)

				Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())

				verifyDeployment(config, isScaleDownDisabled, controllerWorkers)
			},

			Entry("w/o config k8s", emptyConfig, false, runtimeKubernetesVersion),
			Entry("with scale-down disabled", emptyConfig, true, runtimeKubernetesVersion),
			Entry("with non-default autoscaler config", configWithAutoscalerConfig, false, runtimeKubernetesVersion),
			Entry("with feature flags", configWithFeatureFlags, false, runtimeKubernetesVersion),
			Entry("with NodeCIDRMaskSize", configWithNodeCIDRMaskSize, false, runtimeKubernetesVersion),
			Entry("with PodEvictionTimeout", configWithPodEvictionTimeout, false, runtimeKubernetesVersion),
			Entry("with NodeMonitorGracePeriod", configWithNodeMonitorGracePeriod, false, runtimeKubernetesVersion),
		)

		DescribeTable("success tests for various kubernetes versions (workerless shoot)",
			func(config *gardencorev1beta1.KubeControllerManagerConfig, isScaleDownDisabled bool, controllerWorkers ControllerWorkers) {
				isWorkerless = true
				semverVersion, err := semver.NewVersion(version)
				Expect(err).NotTo(HaveOccurred())

				values = Values{
					RuntimeVersion:         runtimeKubernetesVersion,
					TargetVersion:          semverVersion,
					Image:                  image,
					Config:                 config,
					PriorityClassName:      priorityClassName,
					IsScaleDownDisabled:    isScaleDownDisabled,
					IsWorkerless:           isWorkerless,
					PodNetworks:            podCIDRs,
					ServiceNetworks:        serviceCIDRs,
					ClusterSigningDuration: clusterSigningDuration,
					ControllerWorkers:      controllerWorkers,
					ControllerSyncPeriods:  controllerSyncPeriods,
				}
				kubeControllerManager = New(testLogger, fakeInterface, namespace, sm, values)
				kubeControllerManager.SetReplicaCount(replicas)

				Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())

				verifyDeployment(config, isScaleDownDisabled, controllerWorkers)
			},

			Entry("w/o config", emptyConfig, false, controllerWorkers),
			Entry("with scale-down disabled", emptyConfig, true, controllerWorkers),
			Entry("with non-default autoscaler config", configWithAutoscalerConfig, false, controllerWorkers),
			Entry("with feature flags", configWithFeatureFlags, false, controllerWorkers),
			Entry("with NodeCIDRMaskSize", configWithNodeCIDRMaskSize, false, controllerWorkers),
			Entry("with PodEvictionTimeout", configWithPodEvictionTimeout, false, controllerWorkers),
			Entry("with NodeMonitorGracePeriod", configWithNodeMonitorGracePeriod, false, controllerWorkers),
			Entry("with disabled controllers", configWithNodeMonitorGracePeriod, false, controllerWorkersWithDisabledControllers),
		)

		DescribeTable("success tests for various runtime config",
			func(config *gardencorev1beta1.KubeControllerManagerConfig, runtimeConfig map[string]bool, workerless bool, expectedCommand string) {
				semverVersion, err := semver.NewVersion(version)
				Expect(err).NotTo(HaveOccurred())

				values = Values{
					RuntimeVersion:         runtimeKubernetesVersion,
					TargetVersion:          semverVersion,
					Image:                  image,
					Config:                 config,
					PriorityClassName:      priorityClassName,
					IsWorkerless:           workerless,
					PodNetworks:            podCIDRs,
					ServiceNetworks:        serviceCIDRs,
					ClusterSigningDuration: clusterSigningDuration,
					ControllerWorkers:      controllerWorkers,
					ControllerSyncPeriods:  controllerSyncPeriods,
				}
				kubeControllerManager = New(testLogger, fakeInterface, namespace, sm, values)
				kubeControllerManager.SetReplicaCount(replicas)
				kubeControllerManager.SetRuntimeConfig(runtimeConfig)

				Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())

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
						KeepObjects: ptr.To(true),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				actualDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualDeployment), actualDeployment)).To(Succeed())
				Expect(actualDeployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(expectedCommand))
			},

			Entry("with empty runtimeConfig",
				configWithNodeMonitorGracePeriod,
				nil,
				false,
				"--controllers=*,bootstrapsigner,tokencleaner",
			),
			Entry("with empty runtimeConfig (workerless)",
				configWithNodeMonitorGracePeriod,
				nil,
				true,
				"--controllers=*,bootstrapsigner,tokencleaner,-attachdetach,-cloud-node-lifecycle,-endpoint,-ephemeral-volume,-nodeipam,-nodelifecycle,-persistentvolume-binder,-persistentvolume-expander,-pv-protection,-ttl",
			),
			Entry("with disabled APIs (workerless)",
				configWithNodeMonitorGracePeriod,
				map[string]bool{
					"apps/v1": false,
				},
				true,
				"--controllers=*,bootstrapsigner,tokencleaner,-attachdetach,-cloud-node-lifecycle,-daemonset,-deployment,-endpoint,-ephemeral-volume,-nodeipam,-nodelifecycle,-persistentvolume-binder,-persistentvolume-expander,-pv-protection,-replicaset,-statefulset,-ttl",
			),
			Entry("with non-disabled APIs (workerless)",
				configWithNodeMonitorGracePeriod,
				map[string]bool{
					"apps/v1": true,
				},
				true,
				"--controllers=*,bootstrapsigner,tokencleaner,-attachdetach,-cloud-node-lifecycle,-endpoint,-ephemeral-volume,-nodeipam,-nodelifecycle,-persistentvolume-binder,-persistentvolume-expander,-pv-protection,-ttl",
			),
			Entry("with disabled APIs",
				configWithNodeMonitorGracePeriod,
				map[string]bool{
					"autoscaling/v2":                 false,
					"batch/v1":                       false,
					"apps/v1":                        true,
					"policy/v1/poddisruptionbudgets": false,
					"storage.k8s.io/v1/csinodes":     false,
				},
				false,
				"--controllers=*,bootstrapsigner,tokencleaner,-cronjob,-horizontalpodautoscaling,-job,-ttl-after-finished",
			),
			Entry("with disabled APIs",
				configWithNodeMonitorGracePeriod,
				map[string]bool{
					"resource.k8s.io/v1alpha2":           false,
					"discovery.k8s.io/v1":                false,
					"internal.apiserver.k8s.io/v1alpha1": false,
					"rbac.authorization.k8s.io/v1":       false,
				},
				false,
				"--controllers=*,bootstrapsigner,tokencleaner,-clusterrole-aggregation,-endpointslice,-endpointslicemirroring,-resource-claim-controller,-storage-version-gc",
			),
		)

		Context("when name prefix is set", func() {
			BeforeEach(func() {
				values = Values{
					RuntimeVersion:      runtimeKubernetesVersion,
					TargetVersion:       semverVersion,
					Image:               image,
					Config:              &kcmConfig,
					PriorityClassName:   priorityClassName,
					IsScaleDownDisabled: false,
					IsWorkerless:        isWorkerless,
					PodNetworks:         podCIDRs,
					ServiceNetworks:     serviceCIDRs,
					NamePrefix:          "virtual-garden-",
				}
				kubeControllerManager = New(
					testLogger,
					fakeInterface,
					namespace,
					sm,
					values,
				)
			})

			It("should deploy the expected Service", func() {
				Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())

				service.Name = "virtual-garden-kube-controller-manager"
				service.Annotations = map[string]string{"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":10257}]`}

				actualService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: service.Name, Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualService), actualService)).To(Succeed())
				Expect(actualService).To(DeepEqual(service))
			})

			It("should deploy the expected monitoring resources", func() {
				Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())

				expectedServiceMonitor := serviceMonitor("garden", values.NamePrefix)
				actualServiceMonitor := &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: expectedServiceMonitor.Name, Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualServiceMonitor), actualServiceMonitor)).To(Succeed())
				Expect(actualServiceMonitor).To(DeepEqual(expectedServiceMonitor))

				expectedPrometheusRule := prometheusRule("garden", values.NamePrefix)
				actualPrometheusRule := &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: expectedPrometheusRule.Name, Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualPrometheusRule), actualPrometheusRule)).To(Succeed())
				Expect(actualPrometheusRule).To(DeepEqual(expectedPrometheusRule))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}
			vpa := &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace}}
			service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: namespace}}
			serviceMonitor := &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "shoot-kube-controller-manager", Namespace: namespace}}
			prometheusRule := &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: "shoot-kube-controller-manager", Namespace: namespace}}
			pdb := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: pdbName, Namespace: namespace}}
			deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace}}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}}

			Expect(c.Create(ctx, mr)).To(Succeed())
			Expect(c.Create(ctx, vpa)).To(Succeed())
			Expect(c.Create(ctx, service)).To(Succeed())
			Expect(c.Create(ctx, serviceMonitor)).To(Succeed())
			Expect(c.Create(ctx, prometheusRule)).To(Succeed())
			Expect(c.Create(ctx, deploy)).To(Succeed())
			Expect(c.Create(ctx, pdb)).To(Succeed())
			Expect(c.Create(ctx, secret)).To(Succeed())

			kubeControllerManager = New(
				testLogger,
				fakeInterface,
				namespace,
				sm,
				values,
			)

			Expect(kubeControllerManager.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(service), service)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceMonitor), serviceMonitor)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), prometheusRule)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(pdb), pdb)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		var (
			deployment *appsv1.Deployment
			labels     = map[string]string{"role": "kcm"}
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-controller-manager",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{MatchLabels: labels},
				},
			}
		})

		It("should successfully wait for the deployment to be updated", func() {
			values = Values{
				RuntimeVersion: semver.MustParse("1.31.1"),
				IsWorkerless:   isWorkerless,
			}
			kubeControllerManager = New(testLogger, fakeInterface, namespace, nil, values)

			deploy := deployment.DeepCopy()

			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(c.Create(ctx, deploy)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			Expect(c.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: deployment.Namespace,
					Labels:    labels,
				},
			})).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				deploy.Generation = 24
				deploy.Spec.Replicas = ptr.To[int32](1)
				deploy.Status.Conditions = []appsv1.DeploymentCondition{
					{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
					{Type: appsv1.DeploymentAvailable, Status: "True"},
				}
				deploy.Status.ObservedGeneration = deploy.Generation
				deploy.Status.Replicas = *deploy.Spec.Replicas
				deploy.Status.UpdatedReplicas = *deploy.Spec.Replicas
				deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
				Expect(c.Status().Update(ctx, deploy)).To(Succeed())
			})
			defer timer.Stop()

			Expect(kubeControllerManager.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeControllerManager.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

// Utility functions

func commandForKubernetesVersion(
	port int32,
	nodeCIDRMaskSize *int32,
	nodeMonitorGracePeriod *metav1.Duration,
	clusterName string,
	isWorkerless bool,
	serviceNetwork, podNetwork []net.IPNet,
	horizontalPodAutoscalerConfig *gardencorev1beta1.HorizontalPodAutoscalerConfig,
	featureGateFlags string,
	clusterSigningDuration *time.Duration,
	controllerWorkers ControllerWorkers,
	controllerSyncPeriods ControllerSyncPeriods,
) []string {
	var (
		command                       []string
		controllers                   = []string{"*", "bootstrapsigner", "tokencleaner"}
		nodeMonitorGracePeriodSetting = "40s"
	)

	if nodeMonitorGracePeriod != nil {
		nodeMonitorGracePeriodSetting = nodeMonitorGracePeriod.Duration.String()
	}

	command = append(command,
		"/usr/local/bin/kube-controller-manager",
		"--authentication-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--authorization-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
	)

	if !isWorkerless {
		if nodeCIDRMaskSize != nil {
			command = append(command, fmt.Sprintf("--node-cidr-mask-size-ipv4=%d", *nodeCIDRMaskSize))
			command = append(command, fmt.Sprintf("--node-cidr-mask-size-ipv6=%d", 64))
		}

		command = append(command,
			"--allocate-node-cidrs=true",
			"--attach-detach-reconcile-sync-period=1m0s",
			fmt.Sprintf("--cluster-cidr=%s", netutils.JoinByComma(podNetwork)),
			"--cluster-signing-kubelet-client-cert-file=/srv/kubernetes/ca-client/ca.crt",
			"--cluster-signing-kubelet-client-key-file=/srv/kubernetes/ca-client/ca.key",
			"--cluster-signing-kubelet-serving-cert-file=/srv/kubernetes/ca-kubelet/ca.crt",
			"--cluster-signing-kubelet-serving-key-file=/srv/kubernetes/ca-kubelet/ca.key",
			fmt.Sprintf("--horizontal-pod-autoscaler-downscale-stabilization=%s", horizontalPodAutoscalerConfig.DownscaleStabilization.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-initial-readiness-delay=%s", horizontalPodAutoscalerConfig.InitialReadinessDelay.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-cpu-initialization-period=%s", horizontalPodAutoscalerConfig.CPUInitializationPeriod.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-sync-period=%s", horizontalPodAutoscalerConfig.SyncPeriod.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-tolerance=%v", *horizontalPodAutoscalerConfig.Tolerance),
			"--leader-elect=true",
			fmt.Sprintf("--node-monitor-grace-period=%s", nodeMonitorGracePeriodSetting),
		)

		if v := controllerWorkers.Deployment; v == nil {
			command = append(command, "--concurrent-deployment-syncs=50")
		} else {
			command = append(command, fmt.Sprintf("--concurrent-deployment-syncs=%d", *v))
		}

		if v := controllerWorkers.ReplicaSet; v == nil {
			command = append(command, "--concurrent-replicaset-syncs=50")
		} else {
			command = append(command, fmt.Sprintf("--concurrent-replicaset-syncs=%d", *v))
		}

		if v := controllerWorkers.StatefulSet; v == nil {
			command = append(command, "--concurrent-statefulset-syncs=15")
		} else {
			command = append(command, fmt.Sprintf("--concurrent-statefulset-syncs=%d", *v))
		}
	} else {
		controllers = append(controllers,
			"-attachdetach",
			"-cloud-node-lifecycle",
			"-endpoint",
			"-ephemeral-volume",
		)

		if controllerWorkers.Namespace != nil && *controllerWorkers.Namespace == 0 {
			controllers = append(controllers, "-namespace")
		}

		controllers = append(controllers,
			"-nodeipam",
			"-nodelifecycle",
			"-persistentvolume-binder",
			"-persistentvolume-expander",
			"-pv-protection",
		)

		if controllerWorkers.ResourceQuota != nil && *controllerWorkers.ResourceQuota == 0 {
			controllers = append(controllers, "-resourcequota")
		}

		if controllerWorkers.ServiceAccountToken != nil && *controllerWorkers.ServiceAccountToken == 0 {
			controllers = append(controllers, "-serviceaccount-token")
		}

		controllers = append(controllers, "-ttl")
	}

	command = append(command,
		fmt.Sprintf("--cluster-name=%s", clusterName),
		"--cluster-signing-kube-apiserver-client-cert-file=/srv/kubernetes/ca-client/ca.crt",
		"--cluster-signing-kube-apiserver-client-key-file=/srv/kubernetes/ca-client/ca.key",
		"--cluster-signing-legacy-unknown-cert-file=/srv/kubernetes/ca-client/ca.crt",
		"--cluster-signing-legacy-unknown-key-file=/srv/kubernetes/ca-client/ca.key",
	)

	if clusterSigningDuration == nil {
		command = append(command, "--cluster-signing-duration=720h")
	} else {
		command = append(command, "--cluster-signing-duration="+clusterSigningDuration.String())
	}

	if v := controllerWorkers.Endpoint; v == nil {
		command = append(command, "--concurrent-endpoint-syncs=15")
	} else {
		command = append(command, fmt.Sprintf("--concurrent-endpoint-syncs=%d", *v))
	}

	if v := controllerWorkers.GarbageCollector; v == nil {
		command = append(command, "--concurrent-gc-syncs=30")
	} else {
		command = append(command, fmt.Sprintf("--concurrent-gc-syncs=%d", *v))
	}

	if v := controllerWorkers.ServiceEndpoint; v == nil {
		command = append(command, "--concurrent-service-endpoint-syncs=15")
	} else {
		command = append(command, fmt.Sprintf("--concurrent-service-endpoint-syncs=%d", *v))
	}

	command = append(command, "--controllers="+strings.Join(controllers, ","))

	if v := controllerWorkers.Namespace; v == nil {
		command = append(command, "--concurrent-namespace-syncs=50")
	} else if *v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-namespace-syncs=%d", *v))
	}

	if v := controllerWorkers.ResourceQuota; v == nil {
		command = append(command, "--concurrent-resource-quota-syncs=15")
	} else if *v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-resource-quota-syncs=%d", *v))

		if v := controllerSyncPeriods.ResourceQuota; v != nil {
			command = append(command, "--resource-quota-sync-period="+v.String())
		}
	}

	if v := controllerWorkers.ServiceAccountToken; v == nil {
		command = append(command, "--concurrent-serviceaccount-token-syncs=15")
	} else if *v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-serviceaccount-token-syncs=%d", *v))
	}

	if len(featureGateFlags) > 0 {
		command = append(command, featureGateFlags)
	}

	command = append(command,
		"--root-ca-file=/srv/kubernetes/ca/bundle.crt",
		"--service-account-private-key-file=/srv/kubernetes/service-account-key/id_rsa",
		fmt.Sprintf("--secure-port=%d", port),
	)

	if serviceNetwork != nil {
		command = append(command,
			fmt.Sprintf("--service-cluster-ip-range=%s", netutils.JoinByComma(serviceNetwork)),
		)
	}

	command = append(command,
		"--profiling=false",
		"--tls-cert-file=/var/lib/kube-controller-manager-server/tls.crt",
		"--tls-private-key-file=/var/lib/kube-controller-manager-server/tls.key",
	)

	command = append(command, "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305")

	command = append(command,
		"--use-service-account-credentials=true",
		"--v=2",
	)

	return command
}

func getHorizontalPodAutoscalerConfig(config *gardencorev1beta1.HorizontalPodAutoscalerConfig) *gardencorev1beta1.HorizontalPodAutoscalerConfig {
	defaultHPATolerance := gardencorev1beta1.DefaultHPATolerance
	horizontalPodAutoscalerConfig := gardencorev1beta1.HorizontalPodAutoscalerConfig{
		CPUInitializationPeriod: &metav1.Duration{Duration: 5 * time.Minute},
		DownscaleStabilization:  &metav1.Duration{Duration: 5 * time.Minute},
		InitialReadinessDelay:   &metav1.Duration{Duration: 30 * time.Second},
		SyncPeriod:              &metav1.Duration{Duration: 30 * time.Second},
		Tolerance:               &defaultHPATolerance,
	}

	if config != nil {
		if config.CPUInitializationPeriod != nil {
			horizontalPodAutoscalerConfig.CPUInitializationPeriod = config.CPUInitializationPeriod
		}
		if config.DownscaleStabilization != nil {
			horizontalPodAutoscalerConfig.DownscaleStabilization = config.DownscaleStabilization
		}
		if config.InitialReadinessDelay != nil {
			horizontalPodAutoscalerConfig.InitialReadinessDelay = config.InitialReadinessDelay
		}
		if config.SyncPeriod != nil {
			horizontalPodAutoscalerConfig.SyncPeriod = config.SyncPeriod
		}
		if config.Tolerance != nil {
			horizontalPodAutoscalerConfig.Tolerance = config.Tolerance
		}
	}
	return &horizontalPodAutoscalerConfig
}
